package http

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/jackc/pgx/v5/pgxpool"

	"gitlab.com/syncbox/backend/internal/config"
	"gitlab.com/syncbox/backend/internal/domain"
	"gitlab.com/syncbox/backend/internal/http/handler"
	"gitlab.com/syncbox/backend/internal/http/middleware"
	"gitlab.com/syncbox/backend/internal/repository"
	"gitlab.com/syncbox/backend/internal/service"
	"gitlab.com/syncbox/backend/internal/token"
)

// Deps agrupa todas las dependencias inyectadas en el router.
type Deps struct {
	Cfg    *config.Config
	Pool   *pgxpool.Pool
	Tokens *token.Manager

	UserRepo     *repository.UserRepo
	EquipoRepo   *repository.EquipoRepo
	EscenaRepo   *repository.EscenaRepo
	SessionRepo  *repository.SessionRepo
	NodoRepo     *repository.NodoRepo
	Modelo3DRepo *repository.Modelo3DRepo

	AuthSvc     *service.AuthService
	UserSvc     *service.UserService
	CatalogSvc  *service.CatalogService
	EquipoSvc   *service.EquipoService
	AlertaSvc   *service.AlertaService
	EscenaSvc   *service.EscenaService
	NodoSvc     *service.NodoService
	Modelo3DSvc *service.Modelo3DService
}

// NewServer crea la app Fiber con sus middlewares globales.
func NewServer(d *Deps) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "syncbox-backend",
		ErrorHandler: middleware.ErrorHandler,
		BodyLimit:    (d.Cfg.ModelMaxMB + 16) * 1024 * 1024,
	})

	app.Use(recover.New())
	app.Use(fiberlogger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     d.Cfg.CORSOrigin,
		AllowMethods:     "GET,POST,PATCH,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Content-Type,Authorization",
		AllowCredentials: false,
		MaxAge:           600,
	}))

	registerRoutes(app, d)
	return app
}

func registerRoutes(app *fiber.App, d *Deps) {
	health := handler.NewHealthHandler(d.Pool)
	app.Get("/health/live", health.Live)
	app.Get("/health/ready", health.Ready)

	api := app.Group("/api/v1")

	// Públicos
	authH := handler.NewAuthHandler(d.AuthSvc, d.UserRepo)
	api.Post("/auth/login", authH.Login)
	api.Post("/auth/refresh", authH.Refresh)

	// Protegidos
	auth := middleware.RequireAuth(d.Tokens, d.AuthSvc)
	authed := api.Group("", auth)

	authed.Post("/auth/logout", authH.Logout)
	authed.Post("/auth/password", authH.ChangePassword)
	authed.Get("/me", authH.Me)
	authed.Patch("/me/preferencias", authH.UpdatePreferences)

	// Catálogos: lectura para cualquier autenticado, escritura solo Admin.
	cat := handler.NewCatalogHandler(d.CatalogSvc)
	authed.Get("/categorias", cat.ListCategorias)
	authed.Get("/estados", cat.ListEstados)
	adminOnly := middleware.RequireRole(d.UserRepo, domain.RoleAdmin)
	authed.Post("/categorias", adminOnly, cat.CreateCategoria)
	authed.Patch("/categorias/:id", adminOnly, cat.UpdateCategoria)

	// Usuarios — Admin.
	uH := handler.NewUserHandler(d.UserSvc)
	usr := authed.Group("/usuarios", adminOnly)
	usr.Get("/", uH.List)
	usr.Post("/", uH.Create)
	usr.Get("/:id", uH.Get)
	usr.Patch("/:id", uH.Update)
	usr.Delete("/:id", uH.Deactivate)

	// Equipos — Admin u Operador para escritura; lectura para autenticados.
	editor := middleware.RequireRole(d.UserRepo, domain.RoleAdmin, domain.RoleOperator)
	owns := middleware.RequireOwnership(d.EquipoRepo, "id")
	eH := handler.NewEquipoHandler(d.EquipoSvc)
	eq := authed.Group("/equipos")
	eq.Get("/", eH.List)
	eq.Post("/", editor, eH.Create)
	eq.Get("/:id", owns, eH.Get)
	eq.Patch("/:id", editor, owns, eH.Update)
	eq.Delete("/:id", editor, owns, eH.Delete)
	eq.Patch("/:id/modelo3d", editor, owns, eH.SetModelo3D)
	eq.Patch("/:id/estado", editor, owns, eH.ChangeState)
	eq.Get("/:id/historial", owns, eH.Historial)
	eq.Get("/:id/ficha", owns, eH.GetFicha)
	eq.Put("/:id/ficha", editor, owns, eH.UpsertFicha)

	// Alertas — Admin.
	aH := handler.NewAlertaHandler(d.AlertaSvc)
	al := authed.Group("/alertas", editor)
	al.Get("/", aH.List)
	al.Get("/pendientes", aH.Pendientes)
	al.Post("/:id/resolver", aH.Resolver)
	al.Post("/:id/posponer", aH.Posponer)
	al.Post("/:id/visto", aH.MarkVisto)
	authed.Get("/alertas/config", adminOnly, aH.Configuracion)
	authed.Patch("/alertas/config/:estado_id", adminOnly, aH.UpdateConfig)

	// Nodos (árbol UBICACION/LABORATORIO/EQUIPO) — lectura todos, escritura admin.
	nH := handler.NewNodoHandler(d.NodoSvc)
	nodos := authed.Group("/nodos")
	nodos.Get("/", nH.List)
	nodos.Get("/:id", nH.Get)
	nodos.Get("/:id/children", nH.Children)
	nodos.Get("/:id/subtree", nH.Subtree)
	nodos.Get("/:id/ancestors", nH.Ancestors)
	nodos.Post("/", adminOnly, nH.Create)
	nodos.Patch("/:id", adminOnly, nH.Update)
	nodos.Post("/:id/move", adminOnly, nH.Move)
	nodos.Delete("/:id", adminOnly, nH.Delete)

	// Modelos 3D — lectura todos, subida admin/operador, borrado admin.
	mH := handler.NewModelo3DHandler(d.Modelo3DSvc)
	modelos := authed.Group("/modelos3d")
	modelos.Get("/", mH.List)
	modelos.Get("/:id", mH.Get)
	modelos.Get("/:id/file", mH.File)
	modelos.Post("/", editor, mH.Upload)
	modelos.Patch("/:id", adminOnly, mH.Update)
	modelos.Delete("/:id", adminOnly, mH.Delete)

	// Lectura: cualquier autenticado. Crear/eliminar escenas: Admin.
	// Mutar instancias dentro de la escena (mover/escalar/añadir/quitar): editor.
	escH := handler.NewEscenaHandler(d.EscenaSvc)
	esc := authed.Group("/escenas")
	esc.Get("/", escH.List)
	esc.Get("/:id/auditoria", escH.Auditoria)
	esc.Get("/:id", escH.Get)
	esc.Post("/", adminOnly, escH.Create)
	esc.Patch("/:id", adminOnly, escH.Update)
	esc.Patch("/:id/iluminacion", editor, escH.UpdateLighting)
	esc.Post("/:id/sesiones", escH.StartSesion)
	esc.Post("/:id/sesiones/:sid/cerrar", escH.CloseSesion)
	esc.Delete("/:id", adminOnly, escH.Delete)
	esc.Post("/:id/instancias", editor, escH.AddInstancia)
	esc.Patch("/:id/instancias/:iid", editor, escH.UpdateInstancia)
	esc.Post("/:id/instancias/:iid/restore", editor, escH.RestoreInstancia)
	esc.Post("/:id/instancias/:iid/restore-session", editor, escH.RestoreInstanciaFromLastSession)
	esc.Delete("/:id/instancias/:iid", editor, escH.RemoveInstancia)
}
