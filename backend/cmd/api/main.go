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

	"github.com/FedeCF03/nandubay/backend/internal/auth"
	"github.com/FedeCF03/nandubay/backend/internal/config"
	"github.com/FedeCF03/nandubay/backend/internal/db"
	"github.com/FedeCF03/nandubay/backend/internal/httpapi"
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

	pool, err := db.Conectar(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if cfg.Migrar {
		slog.Info("aplicando migraciones")
		if err := db.Migrar(pool); err != nil {
			return err
		}
	}

	srv := &http.Server{
		Addr: ":" + cfg.Puerto,
		Handler: (&httpapi.Servidor{
			DB:           pool,
			Auth:         &auth.Servicio{DB: pool},
			Limitador:    auth.NuevoLimitador(10, 15*time.Minute),
			CORSOrigen:   cfg.CORSOrigen,
			CookieSegura: cfg.CookieSegura,
		}).Router(),
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
