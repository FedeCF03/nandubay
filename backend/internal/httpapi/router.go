// Package httpapi expone la API HTTP.
package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/FedeCF03/nandubay/backend/internal/cortes"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Servidor struct {
	DB         *pgxpool.Pool
	CORSOrigen string
}

func (s *Servidor) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(s.cors)

	r.Get("/healthz", s.salud)
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/cortes/optimizar", s.optimizarCortes)
	})
	return r
}

func (s *Servidor) salud(w http.ResponseWriter, r *http.Request) {
	if err := s.DB.Ping(r.Context()); err != nil {
		responderError(w, http.StatusServiceUnavailable, "la base de datos no responde")
		return
	}
	responderJSON(w, http.StatusOK, map[string]string{"estado": "ok"})
}

type pedidoOptimizar struct {
	Parametros   cortes.Parametros `json:"parametros"`
	Piezas       []cortes.Pieza    `json:"piezas"`
	MermaCantoMM int               `json:"merma_canto_mm"`
}

type respuestaOptimizar struct {
	cortes.Resultado
	MetrosCanto float64 `json:"metros_canto"`
}

// optimizarCortes calcula un plan sin guardarlo. Más adelante se suma la
// versión que lee las piezas de un pedido y guarda el plan.
func (s *Servidor) optimizarCortes(w http.ResponseWriter, r *http.Request) {
	var p pedidoOptimizar
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&p); err != nil {
		responderError(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}
	if len(p.Piezas) == 0 {
		responderError(w, http.StatusBadRequest, "no hay piezas para cortar")
		return
	}
	res, err := cortes.Optimizar(p.Parametros, p.Piezas)
	if err != nil {
		responderError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	responderJSON(w, http.StatusOK, respuestaOptimizar{res, cortes.MetrosCanto(p.Piezas, p.MermaCantoMM)})
}

func (s *Servidor) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.CORSOrigen != "" {
			w.Header().Set("Access-Control-Allow-Origin", s.CORSOrigen)
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

func responderJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func responderError(w http.ResponseWriter, status int, msg string) {
	responderJSON(w, status, map[string]string{"error": msg})
}
