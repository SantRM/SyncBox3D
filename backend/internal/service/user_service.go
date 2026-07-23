package service

import (
	"context"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"gitlab.com/syncbox/backend/internal/crypto"
	"gitlab.com/syncbox/backend/internal/domain"
	"gitlab.com/syncbox/backend/internal/repository"
)

// emailRe es una validación pragmática de correo (RFC 5322 simplificado).
// No pretende ser exhaustiva, solo descartar entradas evidentemente erróneas.
var emailRe = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}$`)

// UserService implementa los casos de uso de gestión de usuarios.
// Solo el rol ADMINISTRADOR puede invocar estas operaciones (validado en el
// middleware de roles); aquí se aplican reglas de negocio adicionales.
type UserService struct {
	users    *repository.UserRepo
	sessions *repository.SessionRepo
	audit    *repository.HistorialRepo
}

// NewUserService construye el servicio.
func NewUserService(users *repository.UserRepo, sessions *repository.SessionRepo, audit *repository.HistorialRepo) *UserService {
	return &UserService{users: users, sessions: sessions, audit: audit}
}

// CreateInput es la entrada para alta de usuario.
type CreateInput struct {
	Nombre   string      `json:"nombre"`
	Correo   string      `json:"correo"`
	Password string      `json:"password"`
	Rol      domain.Role `json:"rol"`
}

// List devuelve todos los usuarios.
func (s *UserService) List(ctx context.Context) ([]domain.PublicUsuario, error) {
	us, err := s.users.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.PublicUsuario, 0, len(us))
	for _, u := range us {
		out = append(out, u.ToPublic())
	}
	return out, nil
}

// Get devuelve un usuario.
func (s *UserService) Get(ctx context.Context, id uuid.UUID) (*domain.PublicUsuario, error) {
	u, err := s.users.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	pub := u.ToPublic()
	return &pub, nil
}

// Create da de alta un usuario.
func (s *UserService) Create(ctx context.Context, actor uuid.UUID, in CreateInput) (*domain.PublicUsuario, error) {
	in.Nombre = strings.TrimSpace(in.Nombre)
	in.Correo = strings.TrimSpace(strings.ToLower(in.Correo))
	if in.Nombre == "" || len(in.Password) < 10 || len(in.Password) > crypto.MaxPasswordBytes || !in.Rol.Valid() {
		return nil, domain.ErrInvalidInput
	}
	if !emailRe.MatchString(in.Correo) {
		return nil, domain.ErrInvalidInput
	}
	hash, err := crypto.HashPassword(in.Password)
	if err != nil {
		return nil, err
	}
	u := &domain.Usuario{
		Nombre: in.Nombre, Correo: in.Correo,
		PasswordHash: hash, Rol: in.Rol, Activo: true,
	}
	if err := s.users.Create(ctx, u, &actor); err != nil {
		return nil, err
	}
	_ = s.audit.Record(ctx, repository.EntUsuario, u.ID, actor, []repository.Mutacion{
		{Campo: "alta", Anterior: "", Nuevo: in.Correo},
		{Campo: "rol", Anterior: "", Nuevo: string(in.Rol)},
	})
	pub := u.ToPublic()
	return &pub, nil
}

// UpdateInput aplica un parche.
type UpdateInput struct {
	Nombre *string      `json:"nombre,omitempty"`
	Rol    *domain.Role `json:"rol,omitempty"`
	Activo *bool        `json:"activo,omitempty"`
}

// Update actualiza un usuario protegiendo el invariante "al menos un admin".
//
// La validación del invariante y la escritura se ejecutan dentro de una
// transacción serializada por advisory lock, evitando una condición de
// carrera: dos administradores que se demoten mutuamente al mismo tiempo
// no pueden dejar el sistema sin administradores.
func (s *UserService) Update(ctx context.Context, actor, id uuid.UUID, in UpdateInput) (*domain.PublicUsuario, error) {
	if in.Rol != nil && !in.Rol.Valid() {
		return nil, domain.ErrInvalidInput
	}
	// El nombre, si se envía, no puede quedar vacío tras trim.
	if in.Nombre != nil {
		trimmed := strings.TrimSpace(*in.Nombre)
		if trimmed == "" {
			return nil, domain.ErrInvalidInput
		}
		in.Nombre = &trimmed
	}
	// Auto-protección: no permitir que el actor altere su propio rol o
	// active=false; siempre debe hacerlo otro administrador.
	if actor == id && (in.Rol != nil || (in.Activo != nil && !*in.Activo)) {
		return nil, domain.ErrForbidden
	}

	var muts []repository.Mutacion
	var updated *domain.Usuario

	err := s.users.WithAdminTx(ctx, func(tx pgx.Tx) error {
		target, err := s.users.FindByIDTx(ctx, tx, id)
		if err != nil {
			return err
		}
		losingAdmin := (in.Rol != nil && *in.Rol != domain.RoleAdmin && target.Rol == domain.RoleAdmin && target.Activo) ||
			(in.Activo != nil && !*in.Activo && target.Rol == domain.RoleAdmin && target.Activo)
		if losingAdmin {
			n, err := s.users.CountActiveAdminsTx(ctx, tx)
			if err != nil {
				return err
			}
			if n <= 1 {
				return domain.ErrLastAdmin
			}
		}
		muts = buildUserMutaciones(target, in)
		if err := s.users.UpdateTx(ctx, tx, id, in.Nombre, in.Rol, in.Activo, actor); err != nil {
			return err
		}
		updated, err = s.users.FindByIDTx(ctx, tx, id)
		return err
	})
	if err != nil {
		return nil, err
	}

	// Side-effects fuera de la transacción (idempotentes).
	if in.Activo != nil && !*in.Activo {
		_ = s.sessions.RevokeAllForUser(ctx, id)
	}
	if in.Rol != nil {
		// El cambio de rol invalida sesiones para forzar re-login con el rol nuevo.
		_ = s.sessions.RevokeAllForUser(ctx, id)
	}
	_ = s.audit.Record(ctx, repository.EntUsuario, id, actor, muts)
	pub := updated.ToPublic()
	return &pub, nil
}

// Deactivate equivale a un soft-delete. Protege el invariante de admins.
func (s *UserService) Deactivate(ctx context.Context, actor, id uuid.UUID) error {
	// Auto-protección: un actor no puede desactivarse a sí mismo.
	if actor == id {
		return domain.ErrForbidden
	}
	err := s.users.WithAdminTx(ctx, func(tx pgx.Tx) error {
		target, err := s.users.FindByIDTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if !target.Activo {
			// Idempotente: ya está inactivo.
			return nil
		}
		if target.Rol == domain.RoleAdmin {
			n, err := s.users.CountActiveAdminsTx(ctx, tx)
			if err != nil {
				return err
			}
			if n <= 1 {
				return domain.ErrLastAdmin
			}
		}
		return s.users.SetActiveTx(ctx, tx, id, false, actor)
	})
	if err != nil {
		return err
	}
	_ = s.sessions.RevokeAllForUser(ctx, id)
	_ = s.audit.Record(ctx, repository.EntUsuario, id, actor, []repository.Mutacion{
		{Campo: "activo", Anterior: "true", Nuevo: "false"},
	})
	return nil
}

func buildUserMutaciones(before *domain.Usuario, in UpdateInput) []repository.Mutacion {
	var muts []repository.Mutacion
	if in.Nombre != nil && *in.Nombre != before.Nombre {
		muts = append(muts, repository.Mutacion{Campo: "nombre", Anterior: before.Nombre, Nuevo: *in.Nombre})
	}
	if in.Rol != nil && *in.Rol != before.Rol {
		muts = append(muts, repository.Mutacion{Campo: "rol", Anterior: string(before.Rol), Nuevo: string(*in.Rol)})
	}
	if in.Activo != nil && *in.Activo != before.Activo {
		muts = append(muts, repository.Mutacion{Campo: "activo", Anterior: boolStr(before.Activo), Nuevo: boolStr(*in.Activo)})
	}
	return muts
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
