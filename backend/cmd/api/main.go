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

	"github.com/FedeCF03/nandubay/backend/internal/config"
	"github.com/FedeCF03/nandubay/backend/internal/handlers"
	"github.com/FedeCF03/nandubay/backend/internal/repositorios"
	"github.com/FedeCF03/nandubay/backend/internal/seguridad"
	"github.com/FedeCF03/nandubay/backend/internal/servicios"
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

	pool, err := repositorios.Conectar(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if cfg.Migrar {
		slog.Info("aplicando migraciones")
		if err := repositorios.Migrar(pool); err != nil {
			return err
		}
	}

	// Armado: repositorios -> servicios -> handlers.
	casosAcceso := &servicios.ServicioAcceso{
		Repo:   &repositorios.RepoAcceso{DB: pool},
		Hasher: &seguridad.Bcrypt{},
		Tokens: seguridad.Tokens{},
	}
	servidor := &handlers.Servidor{
		Acceso:       casosAcceso,
		Cortes:       servicios.ServicioCortes{Repo: &repositorios.RepoCortes{DB: pool}},
		Stock:        &servicios.ServicioStock{Repo: &repositorios.RepoStock{DB: pool}},
		Ventas:       &servicios.ServicioVentas{Repo: &repositorios.RepoVentas{DB: pool}},
		Salud:        pool.Ping,
		Limitador:    handlers.NuevoLimitador(10, 15*time.Minute),
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
