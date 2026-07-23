package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"

	"gitlab.com/syncbox/backend/internal/config"
	apphttp "gitlab.com/syncbox/backend/internal/http"
	"gitlab.com/syncbox/backend/internal/jobs"
	"gitlab.com/syncbox/backend/internal/logger"
	"gitlab.com/syncbox/backend/internal/repository"
	"gitlab.com/syncbox/backend/internal/service"
	"gitlab.com/syncbox/backend/internal/token"
)

func main() {
	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("fatal")
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger.Init(cfg.LogLevel, cfg.AppEnv)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Persistencia.
	pool, err := repository.Connect(rootCtx, cfg.DBDSN)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Repositorios.
	users := repository.NewUserRepo(pool)
	sessions := repository.NewSessionRepo(pool)
	attempts := repository.NewLoginAttemptRepo(pool)
	categorias := repository.NewCategoriaRepo(pool)
	estados := repository.NewEstadoRepo(pool)
	equipos := repository.NewEquipoRepo(pool)
	alertas := repository.NewAlertaRepo(pool)
	historial := repository.NewHistorialRepo(pool)
	fichas := repository.NewFichaRepo(pool)
	escenas := repository.NewEscenaRepo(pool)
	nodos := repository.NewNodoRepo(pool)
	modelos3d := repository.NewModelo3DRepo(pool)

	// Servicios.
	tokens := token.NewManager(cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL, cfg.JWTIssuer, cfg.JWTAudience)
	authSvc := service.NewAuthService(users, sessions, attempts, tokens, cfg.LoginMaxFails, cfg.LoginBlockTTL)
	userSvc := service.NewUserService(users, sessions, historial)
	catalogSvc := service.NewCatalogService(categorias, estados, historial)
	equipoSvc := service.NewEquipoService(equipos, categorias, estados, historial, fichas, nodos, modelos3d)
	alertaSvc := service.NewAlertaService(alertas)
	escenaSvc := service.NewEscenaService(escenas, equipos, categorias, nodos)
	nodoSvc := service.NewNodoService(nodos)
	modelo3dSvc := service.NewModelo3DService(modelos3d, cfg.ResourceRoot, cfg.ModelMaxMB)

	// Job periódico de alertas.
	alertJob, err := jobs.NewAlertJob(alertaSvc)
	if err != nil {
		return err
	}
	if err := alertJob.Start(); err != nil {
		return err
	}
	defer alertJob.Stop()

	// HTTP.
	app := apphttp.NewServer(&apphttp.Deps{
		Cfg:          cfg,
		Pool:         pool.Pool,
		Tokens:       tokens,
		UserRepo:     users,
		EquipoRepo:   equipos,
		EscenaRepo:   escenas,
		SessionRepo:  sessions,
		NodoRepo:     nodos,
		Modelo3DRepo: modelos3d,
		AuthSvc:      authSvc,
		UserSvc:      userSvc,
		CatalogSvc:   catalogSvc,
		EquipoSvc:    equipoSvc,
		AlertaSvc:    alertaSvc,
		EscenaSvc:    escenaSvc,
		NodoSvc:      nodoSvc,
		Modelo3DSvc:  modelo3dSvc,
	})

	// Arranque del servidor en goroutine para soportar shutdown.
	srvErr := make(chan error, 1)
	go func() {
		log.Info().Str("port", cfg.AppPort).Str("env", cfg.AppEnv).Msg("api iniciada")
		if err := app.Listen(":" + cfg.AppPort); err != nil && !errors.Is(err, http.ErrServerClosed) {
			srvErr <- err
		}
	}()

	// Graceful shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case sig := <-stop:
		log.Info().Stringer("sig", sig).Msg("apagando…")
	case err := <-srvErr:
		return err
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("error al apagar")
	}
	return nil
}
