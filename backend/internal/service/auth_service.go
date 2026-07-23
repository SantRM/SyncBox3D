package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"gitlab.com/syncbox/backend/internal/crypto"
	"gitlab.com/syncbox/backend/internal/domain"
	"gitlab.com/syncbox/backend/internal/repository"
	"gitlab.com/syncbox/backend/internal/token"
)

// AuthService implementa los casos de uso de autenticación.
type AuthService struct {
	users    *repository.UserRepo
	sessions *repository.SessionRepo
	attempts *repository.LoginAttemptRepo
	tokens   *token.Manager

	maxFails int
	blockTTL time.Duration

	// dummyHash es un hash bcrypt válido precomputado al arranque para igualar
	// el coste de Verify cuando el correo no existe (mitigación de timing-attack
	// que delata correos válidos vs inválidos). Su contraseña en claro nunca
	// se persiste; queda solo en memoria y nadie podrá adivinarla.
	dummyHash string
}

// NewAuthService construye el servicio. Genera al arranque un hash bcrypt
// válido con sal aleatoria que se utiliza como señuelo para mantener tiempo
// de respuesta constante en el login.
func NewAuthService(
	users *repository.UserRepo,
	sessions *repository.SessionRepo,
	attempts *repository.LoginAttemptRepo,
	tokens *token.Manager,
	maxFails int,
	blockTTL time.Duration,
) *AuthService {
	dummy, err := crypto.HashPassword(uuid.NewString())
	if err != nil {
		// HashPassword solo puede fallar por un fallo grave (OOM o cost
		// inválido) que invalida la garantía de tiempo constante. Fail-fast
		// es preferible a degradar a un hash malformado que bcrypt rechazaría
		// instantáneamente, reabriendo el timing-attack de enumeración.
		panic("auth: no se pudo precomputar dummyHash: " + err.Error())
	}
	return &AuthService{
		users: users, sessions: sessions, attempts: attempts, tokens: tokens,
		maxFails: maxFails, blockTTL: blockTTL, dummyHash: dummy,
	}
}

// AuthResult contiene los tokens emitidos tras login o refresh.
type AuthResult struct {
	AccessToken      string               `json:"access_token"`
	RefreshToken     string               `json:"refresh_token"`
	AccessExpiresAt  time.Time            `json:"access_expires_at"`
	RefreshExpiresAt time.Time            `json:"refresh_expires_at"`
	User             domain.PublicUsuario `json:"user"`
}

// Login autentica un usuario por correo+contraseña, registra el intento y, si
// es válido, emite par de tokens (access + refresh) con jti persistido.
//
// Aplica anti–brute-force a dos niveles:
//   - per-correo: bloquea tras maxFails fallos consecutivos.
//   - per-IP:     bloquea tras maxFails*4 fallos en la ventana, sin importar
//     el correo, para detectar enumeración masiva.
//
// La respuesta a credenciales inválidas, cuenta inactiva, cuenta bloqueada y
// correo inexistente es deliberadamente uniforme (`ErrInvalidCredential`)
// para evitar enumeración de cuentas válidas. El motivo real se persiste en
// `intento_login` y queda disponible para el operador.
func (s *AuthService) Login(ctx context.Context, correo, password, ip string) (*AuthResult, error) {
	correo = strings.TrimSpace(strings.ToLower(correo))
	if correo == "" || password == "" {
		return nil, domain.ErrInvalidCredential
	}

	since := time.Now().Add(-s.blockTTL)
	fails, err := s.attempts.CountFails(ctx, correo, since)
	if err != nil {
		return nil, err
	}
	if fails >= s.maxFails {
		// IMPORTANTE: no registramos intentos adicionales mientras la cuenta
		// esté bloqueada. De lo contrario un atacante con solo el correo
		// podría extender la ventana de bloqueo indefinidamente (DoS).
		// Devolvemos credenciales inválidas para no enumerar cuentas activas.
		return nil, domain.ErrInvalidCredential
	}
	ipFails, err := s.attempts.CountFailsByIP(ctx, ip, since)
	if err != nil {
		return nil, err
	}
	if ipFails >= s.maxFails*4 {
		// Throttle a nivel de IP: no se registra para no extender la ventana.
		return nil, domain.ErrInvalidCredential
	}

	u, err := s.users.FindByEmail(ctx, correo)
	if err != nil {
		// Hash válido precomputado para igualar tiempos de respuesta.
		_ = crypto.VerifyPassword(s.dummyHash, password)
		_ = s.attempts.Record(ctx, correo, ip, false)
		return nil, domain.ErrInvalidCredential
	}
	if !u.Activo {
		// Igualamos la respuesta (no exponemos "cuenta inactiva" para evitar
		// enumeración). Igual hacemos un VerifyPassword falso para tiempo cte.
		_ = crypto.VerifyPassword(s.dummyHash, password)
		_ = s.attempts.Record(ctx, correo, ip, false)
		return nil, domain.ErrInvalidCredential
	}
	if err := crypto.VerifyPassword(u.PasswordHash, password); err != nil {
		_ = s.attempts.Record(ctx, correo, ip, false)
		return nil, domain.ErrInvalidCredential
	}

	_ = s.attempts.Record(ctx, correo, ip, true)
	_ = s.users.TouchLastLogin(ctx, u.ID, time.Now().UTC())

	return s.issuePair(ctx, u)
}

// Refresh rota un refresh token: revoca el jti viejo y emite uno nuevo.
func (s *AuthService) Refresh(ctx context.Context, refresh string) (*AuthResult, error) {
	claims, err := s.tokens.Parse(refresh)
	if err != nil {
		return nil, err
	}
	if claims.Kind != token.KindRefresh {
		return nil, domain.ErrTokenInvalid
	}
	active, err := s.sessions.IsActive(ctx, claims.JTI)
	if err != nil || !active {
		return nil, domain.ErrTokenRevoked
	}
	u, err := s.users.FindByID(ctx, claims.UserID)
	if err != nil {
		// Si el usuario fue borrado/no existe, revocamos todas las sesiones
		// asociadas al jti para no dejar tokens huérfanos vivos.
		_ = s.sessions.Revoke(ctx, claims.JTI)
		return nil, err
	}
	if !u.Activo {
		// Cuenta inactiva: revocar TODAS las sesiones del usuario para que
		// el refresh no continúe renovándose hasta su expiración natural.
		_ = s.sessions.RevokeAllForUser(ctx, u.ID)
		return nil, domain.ErrAccountInactive
	}
	if err := s.sessions.Revoke(ctx, claims.JTI); err != nil {
		return nil, err
	}
	return s.issuePair(ctx, u)
}

// LogoutAll revoca todas las sesiones del usuario.
func (s *AuthService) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	return s.sessions.RevokeAllForUser(ctx, userID)
}

// IsValidUser garantiza que el usuario sigue activo.
func (s *AuthService) IsValidUser(ctx context.Context, userID uuid.UUID) error {
	u, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if !u.Activo {
		return domain.ErrAccountInactive
	}
	return nil
}

// ChangePassword cambia la contraseña del usuario y revoca todas sus sesiones.
func (s *AuthService) ChangePassword(ctx context.Context, userID uuid.UUID, oldPwd, newPwd string) error {
	if len(newPwd) < 10 || len(newPwd) > crypto.MaxPasswordBytes {
		return domain.ErrInvalidInput
	}
	if oldPwd == newPwd {
		return domain.ErrInvalidInput
	}
	u, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if !u.Activo {
		return domain.ErrAccountInactive
	}
	if err := crypto.VerifyPassword(u.PasswordHash, oldPwd); err != nil {
		return domain.ErrInvalidCredential
	}
	hash, err := crypto.HashPassword(newPwd)
	if err != nil {
		return err
	}
	if err := s.users.UpdatePassword(ctx, userID, hash); err != nil {
		return err
	}
	return s.sessions.RevokeAllForUser(ctx, userID)
}

// IsSessionActive expone al middleware la verificación de revocación.
func (s *AuthService) IsSessionActive(ctx context.Context, jti string) (bool, error) {
	return s.sessions.IsActive(ctx, jti)
}

func (s *AuthService) issuePair(ctx context.Context, u *domain.Usuario) (*AuthResult, error) {
	access, _, accessExp, err := s.tokens.Issue(u.ID, u.Rol, token.KindAccess)
	if err != nil {
		return nil, err
	}
	refresh, refreshJTI, refreshExp, err := s.tokens.Issue(u.ID, u.Rol, token.KindRefresh)
	if err != nil {
		return nil, err
	}
	if err := s.sessions.Create(ctx, refreshJTI, u.ID, refreshExp); err != nil {
		return nil, err
	}
	return &AuthResult{
		AccessToken:      access,
		RefreshToken:     refresh,
		AccessExpiresAt:  accessExp,
		RefreshExpiresAt: refreshExp,
		User:             u.ToPublic(),
	}, nil
}
