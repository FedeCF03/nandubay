// Package config lee la configuración desde variables de entorno.
package config

import (
	"fmt"
	"os"
)

type Config struct {
	Puerto      string // PUERTO, por defecto 8080
	DatabaseURL string // DATABASE_URL, ej: postgres://app:clave@db:5432/nandubay
	Migrar      bool   // MIGRAR=1 corre las migraciones al arrancar
	CORSOrigen  string // CORS_ORIGEN, ej: http://localhost:5173 en desarrollo
}

func Cargar() (Config, error) {
	c := Config{
		Puerto:      envOr("PUERTO", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Migrar:      os.Getenv("MIGRAR") == "1",
		CORSOrigen:  os.Getenv("CORS_ORIGEN"),
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("falta DATABASE_URL")
	}
	return c, nil
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
