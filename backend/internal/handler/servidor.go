// Package api es el adaptador HTTP: traduce pedidos HTTP a casos de uso de la
// capa de aplicación y sus resultados a JSON. No tiene reglas de negocio.
package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/FedeCF03/nandubay/backend/internal/model/acceso"
	"github.com/FedeCF03/nandubay/backend/internal/model/cortes"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// CasosAcceso son los casos de uso de login y sesiones que usa la API.
type CasosAcceso interface {
	Login(ctx context.Context, email, clave, ip, agente string) (string, error)
	Logout(ctx context.Context, token string) error
	Autenticar(ctx context.Context, token string) (*acceso.Usuario, error)
	ElegirSucursal(ctx context.Context, u *acceso.Usuario, token string, sucursalID int64) error
}

// CasosCortes son los casos de uso del optimizador.
type CasosCortes interface {
	Optimizar(p cortes.Parametros, piezas []cortes.Pieza, mermaCantoMM int) (cortes.PlanCortes, error)
	ListarCortes(ctx context.Context, u *acceso.Usuario, estado string) ([]cortes.CorteOptimizado, error)
	VerCorte(ctx context.Context, u *acceso.Usuario, id int64) (cortes.CorteOptimizado, error)
	CrearCorte(ctx context.Context, u *acceso.Usuario, n cortes.CorteNuevo) (cortes.CorteOptimizado, error)
	Recalcular(ctx context.Context, u *acceso.Usuario, id int64) (cortes.CorteOptimizado, error)
	ConfirmarCorte(ctx context.Context, u *acceso.Usuario, id int64) (cortes.CorteOptimizado, error)
	AnularCorte(ctx context.Context, u *acceso.Usuario, id int64) (cortes.CorteOptimizado, error)
}

type Servidor struct {
	Acceso       CasosAcceso
	Cortes       CasosCortes
	Stock        CasosStock
	Ventas       CasosVentas
	Salud        func(context.Context) error // ej: ping a la base
	Limitador    *Limitador
	CORSOrigen   string
	CookieSegura bool
}

func (s *Servidor) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(s.cors)
	r.Use(soloJSON)

	r.Get("/healthz", s.salud)
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/login", s.login)
		r.Post("/auth/logout", s.logout)

		r.Group(func(r chi.Router) {
			r.Use(s.requiereSesion)
			r.Get("/auth/yo", s.yo)
			r.Post("/auth/sucursal", s.elegirSucursal)
			s.rutasCortes(r)
			s.rutasStock(r)
			s.rutasVentas(r)
		})
	})
	return r
}

func (s *Servidor) salud(w http.ResponseWriter, r *http.Request) {
	if s.Salud != nil {
		if err := s.Salud(r.Context()); err != nil {
			responderError(w, http.StatusServiceUnavailable, "la base de datos no responde")
			return
		}
	}
	responderJSON(w, http.StatusOK, map[string]string{"estado": "ok"})
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

func leerJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		responderError(w, http.StatusBadRequest, "JSON inválido")
		return false
	}
	return true
}

func responderJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func responderError(w http.ResponseWriter, status int, msg string) {
	responderJSON(w, status, map[string]string{"error": msg})
}

func errorInterno(w http.ResponseWriter, r *http.Request, err error) {
	slog.ErrorContext(r.Context(), "error interno", "ruta", r.URL.Path, "error", err)
	responderError(w, http.StatusInternalServerError, "algo salió mal, probá de nuevo")
}
