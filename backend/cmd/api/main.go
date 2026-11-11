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
	"github.com/FedeCF03/nandubay/backend/internal/handler"
	"github.com/FedeCF03/nandubay/backend/internal/seguridad"
	"github.com/FedeCF03/nandubay/backend/internal/service"
	"github.com/FedeCF03/nandubay/backend/internal/store"
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

	pool, err := store.Conectar(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if cfg.Migrar {
		slog.Info("aplicando migraciones")
		if err := store.Migrar(pool); err != nil {
			return err
		}
	}

	// Armado: store (Postgres) -> service -> handler.
	casosAcceso := &service.ServicioAcceso{
		Store:  &store.Acceso{DB: pool},
		Hasher: &seguridad.Bcrypt{},
		Tokens: seguridad.Tokens{},
	}
	servidor := &handler.Servidor{
		Acceso:       casosAcceso,
		Cortes:       service.ServicioCortes{Store: &store.Cortes{DB: pool}},
		Stock:        &service.ServicioStock{Store: &store.Stock{DB: pool}},
		Ventas:       &service.ServicioVentas{Store: &store.Ventas{DB: pool}},
		Salud:        pool.Ping,
		Limitador:    handler.NuevoLimitador(10, 15*time.Minute),
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
