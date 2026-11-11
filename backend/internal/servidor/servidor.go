// Package servidor arma el router HTTP con los handler de cada módulo.
package servidor

import (
	"context"
	"net/http"
	"time"

	"github.com/FedeCF03/nandubay/backend/internal/acceso"
	"github.com/FedeCF03/nandubay/backend/internal/cortes"
	"github.com/FedeCF03/nandubay/backend/internal/stock"
	"github.com/FedeCF03/nandubay/backend/internal/ventas"
	"github.com/FedeCF03/nandubay/backend/internal/web"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Servidor struct {
	Acceso     *acceso.Handler
	Cortes     *cortes.Handler
	Stock      *stock.Handler
	Ventas     *ventas.Handler
	Salud      func(context.Context) error // ej: ping a la base
	CORSOrigen string
}

func (s *Servidor) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(s.cors)
	r.Use(web.SoloJSON)

	r.Get("/healthz", s.salud)
	r.Route("/api/v1", func(r chi.Router) {
		s.Acceso.RutasPublicas(r)

		r.Group(func(r chi.Router) {
			r.Use(s.Acceso.RequiereSesion)
			s.Acceso.Rutas(r)
			s.Cortes.Rutas(r)
			s.Stock.Rutas(r)
			s.Ventas.Rutas(r)
		})
	})
	return r
}

func (s *Servidor) salud(w http.ResponseWriter, r *http.Request) {
	if s.Salud != nil {
		if err := s.Salud(r.Context()); err != nil {
			web.ResponderError(w, http.StatusServiceUnavailable, "la base de datos no responde")
			return
		}
	}
	web.ResponderJSON(w, http.StatusOK, map[string]string{"estado": "ok"})
}

func (s *Servidor) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.CORSOrigen != "" {
			w.Header().Set("Access-Control-Allow-Origin", s.CORSOrigen)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
