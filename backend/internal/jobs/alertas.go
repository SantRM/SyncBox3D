package jobs

import (
	"context"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/rs/zerolog/log"

	"gitlab.com/syncbox/backend/internal/service"
)

// AlertJob recorre periódicamente la BD generando alertas según los umbrales
// configurados. Es idempotente gracias al UNIQUE de `alerta_evento`.
type AlertJob struct {
	svc       *service.AlertaService
	scheduler gocron.Scheduler
}

// NewAlertJob construye el job pero no lo arranca.
func NewAlertJob(svc *service.AlertaService) (*AlertJob, error) {
	s, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}
	return &AlertJob{svc: svc, scheduler: s}, nil
}

// Start agenda la ejecución cada hora y arranca el scheduler.
func (j *AlertJob) Start() error {
	_, err := j.scheduler.NewJob(
		gocron.DurationJob(time.Hour),
		gocron.NewTask(j.run),
		gocron.WithStartAt(gocron.WithStartImmediately()),
	)
	if err != nil {
		return err
	}
	j.scheduler.Start()
	return nil
}

// Stop detiene el scheduler liberando goroutines.
func (j *AlertJob) Stop() {
	if j.scheduler != nil {
		_ = j.scheduler.Shutdown()
	}
}

func (j *AlertJob) run() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	n, err := j.svc.GenerarPendientes(ctx)
	if err != nil {
		log.Error().Err(err).Msg("alert_job: error al generar alertas")
		return
	}
	if n > 0 {
		log.Info().Int("alertas_creadas", n).Msg("alert_job: nuevas alertas")
	}
}
