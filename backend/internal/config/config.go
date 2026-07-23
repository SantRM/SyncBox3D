package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config agrupa toda la configuración del proceso. Se carga una sola vez al
// arrancar; cualquier cambio requiere reinicio del contenedor.
type Config struct {
	AppEnv  string
	AppPort string

	DBDSN string

	JWTSecret     []byte
	JWTAccessTTL  time.Duration
	JWTRefreshTTL time.Duration
	JWTIssuer     string
	JWTAudience   string

	LoginMaxFails int
	LoginBlockTTL time.Duration

	CORSOrigin string
	LogLevel   string

	ResourceRoot string
	ModelMaxMB   int
}

// Load lee variables de entorno y aplica defaults seguros para desarrollo.
// JWT_SECRET y DB_DSN son obligatorias y se valida su presencia.
func Load() (*Config, error) {
	_ = godotenv.Load() // best-effort, en producción se inyecta por env

	cfg := &Config{
		AppEnv:        getenv("APP_ENV", "dev"),
		AppPort:       getenv("APP_PORT", "8080"),
		DBDSN:         os.Getenv("DB_DSN"),
		JWTIssuer:     getenv("JWT_ISSUER", "syncbox"),
		JWTAudience:   getenv("JWT_AUDIENCE", "syncbox-app"),
		LoginMaxFails: getenvInt("LOGIN_MAX_FAILS", 5),
		LoginBlockTTL: time.Duration(getenvInt("LOGIN_BLOCK_MIN", 15)) * time.Minute,
		CORSOrigin:    getenv("CORS_ORIGIN", "http://localhost:5173"),
		LogLevel:      getenv("LOG_LEVEL", "info"),
		ResourceRoot:  getenv("RESOURCE_ROOT", "/data/syncbox/recursos"),
		ModelMaxMB:    getenvInt("MODEL_MAX_MB", 500),
	}

	cfg.JWTAccessTTL = time.Duration(getenvInt("JWT_ACCESS_TTL_MIN", 30)) * time.Minute
	cfg.JWTRefreshTTL = time.Duration(getenvInt("JWT_REFRESH_TTL_HOURS", 168)) * time.Hour

	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET requerido, mínimo 32 bytes (actual: %d)", len(secret))
	}
	cfg.JWTSecret = []byte(secret)

	if cfg.DBDSN == "" {
		return nil, fmt.Errorf("DB_DSN requerido")
	}

	return cfg, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getenvInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
