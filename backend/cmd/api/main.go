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

	"github.com/FedeCF03/nandubay/backend/internal/acceso"
	"github.com/FedeCF03/nandubay/backend/internal/config"
	"github.com/FedeCF03/nandubay/backend/internal/cortes"
	"github.com/FedeCF03/nandubay/backend/internal/db"
	"github.com/FedeCF03/nandubay/backend/internal/seguridad"
	"github.com/FedeCF03/nandubay/backend/internal/servidor"
	"github.com/FedeCF03/nandubay/backend/internal/stock"
	"github.com/FedeCF03/nandubay/backend/internal/ventas"
	"github.com/FedeCF03/nandubay/backend/internal/web"
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

	// Armado de cada módulo: PostgresStore -> Servicio -> Handler.
	servidor := &servidor.Servidor{
		Acceso: &acceso.Handler{
			Servicio: &acceso.Servicio{
				Store:  &acceso.PostgresStore{DB: pool},
				Hasher: &seguridad.Bcrypt{},
				Tokens: seguridad.Tokens{},
			},
			Limitador:    web.NuevoLimitador(10, 15*time.Minute),
			CookieSegura: cfg.CookieSegura,
		},
		Cortes:     &cortes.Handler{Servicio: &cortes.Servicio{Store: &cortes.PostgresStore{DB: pool}}},
		Stock:      &stock.Handler{Servicio: &stock.Servicio{Store: &stock.PostgresStore{DB: pool}}},
		Ventas:     &ventas.Handler{Servicio: &ventas.Servicio{Store: &ventas.PostgresStore{DB: pool}}},
		Salud:      pool.Ping,
		CORSOrigen: cfg.CORSOrigen,
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
