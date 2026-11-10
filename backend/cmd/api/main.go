package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // zona horaria de Argentina aunque la imagen no la traiga

	"github.com/FedeCF03/nandubay/backend/internal/adaptadores/api"
	"github.com/FedeCF03/nandubay/backend/internal/adaptadores/postgres"
	"github.com/FedeCF03/nandubay/backend/internal/aplicacion"
	"github.com/FedeCF03/nandubay/backend/internal/plataforma/config"
	"github.com/FedeCF03/nandubay/backend/internal/plataforma/seguridad"
)

func main() {
	if err := correr(); err != nil {
		slog.Error("la API terminó con error", "error", err)
		os.Exit(1)
	}
}

func correr() error {
	cfg, err := config.Cargar()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.Conectar(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if cfg.Migrar {
		slog.Info("aplicando migraciones")
		if err := postgres.Migrar(pool); err != nil {
			return err
		}
	}

	// Armado de las capas: adaptadores -> casos de uso -> API.
	casosAcceso := &aplicacion.ServicioAcceso{
		Repo:   &postgres.RepoAcceso{DB: pool},
		Hasher: &seguridad.Bcrypt{},
		Tokens: seguridad.Tokens{},
	}
	servidor := &api.Servidor{
		Acceso:       casosAcceso,
		Cortes:       aplicacion.ServicioCortes{},
		Stock:        &aplicacion.ServicioStock{Repo: &postgres.RepoStock{DB: pool}},
		Ventas:       &aplicacion.ServicioVentas{Repo: &postgres.RepoVentas{DB: pool}},
		Salud:        pool.Ping,
		Limitador:    api.NuevoLimitador(10, 15*time.Minute),
		CORSOrigen:   cfg.CORSOrigen,
		CookieSegura: cfg.CookieSegura,
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Puerto,
		Handler:           servidor.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		apagar, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(apagar)
	}()
	slog.Info("API escuchando", "puerto", cfg.Puerto)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
