package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/FedeCF03/nandubay/backend/internal/modelos/acceso"
	"github.com/FedeCF03/nandubay/backend/internal/modelos/cortes"
	"github.com/FedeCF03/nandubay/backend/internal/modelos/stock"
	"github.com/go-chi/chi/v5"
)

func (s *Servidor) rutasCortes(r chi.Router) {
	r.With(requierePermiso("cortes.generar")).Post("/cortes/optimizar", s.optimizarCortes)
	r.With(requierePermiso("cortes.ver")).Get("/cortes", s.listarCortes)
	r.With(requierePermiso("cortes.ver")).Get("/cortes/{id}", s.verCorte)
	r.With(requierePermiso("cortes.generar")).Post("/cortes", s.crearCorte)
	r.With(requierePermiso("cortes.generar")).Post("/cortes/{id}/recalcular", s.accionCorte(func(ctx context.Context, u *acceso.Usuario, id int64) (cortes.CorteOptimizado, error) {
		return s.Cortes.Recalcular(ctx, u, id)
	}))
	r.With(requierePermiso("cortes.generar")).Post("/cortes/{id}/anular", s.accionCorte(func(ctx context.Context, u *acceso.Usuario, id int64) (cortes.CorteOptimizado, error) {
		return s.Cortes.AnularCorte(ctx, u, id)
	}))
	r.With(requierePermiso("cortes.confirmar")).Post("/cortes/{id}/confirmar", s.accionCorte(func(ctx context.Context, u *acceso.Usuario, id int64) (cortes.CorteOptimizado, error) {
		return s.Cortes.ConfirmarCorte(ctx, u, id)
	}))
}

type pedidoOptimizar struct {
	Parametros   cortes.Parametros `json:"parametros"`
	Piezas       []cortes.Pieza    `json:"piezas"`
	MermaCantoMM int               `json:"merma_canto_mm"`
}

func (s *Servidor) optimizarCortes(w http.ResponseWriter, r *http.Request) {
	var p pedidoOptimizar
	if !leerJSON(w, r, &p) {
		return
	}
	if len(p.Piezas) == 0 {
		responderError(w, http.StatusBadRequest, "no hay piezas para cortar")
		return
	}
	plan, err := s.Cortes.Optimizar(p.Parametros, p.Piezas, p.MermaCantoMM)
	if err != nil {
		responderError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	responderJSON(w, http.StatusOK, plan)
}

func (s *Servidor) listarCortes(w http.ResponseWriter, r *http.Request) {
	cs, err := s.Cortes.ListarCortes(r.Context(), UsuarioDe(r.Context()), r.URL.Query().Get("estado"))
	if err != nil {
		responderErrorCortes(w, r, err)
		return
	}
	responderJSON(w, http.StatusOK, cs)
}

func (s *Servidor) verCorte(w http.ResponseWriter, r *http.Request) {
	id, ok := idDe(w, r)
	if !ok {
		return
	}
	c, err := s.Cortes.VerCorte(r.Context(), UsuarioDe(r.Context()), id)
	if err != nil {
		responderErrorCortes(w, r, err)
		return
	}
	responderJSON(w, http.StatusOK, c)
}

func (s *Servidor) crearCorte(w http.ResponseWriter, r *http.Request) {
	var n cortes.CorteNuevo
	if !leerJSON(w, r, &n) {
		return
	}
	c, err := s.Cortes.CrearCorte(r.Context(), UsuarioDe(r.Context()), n)
	if err != nil {
		responderErrorCortes(w, r, err)
		return
	}
	responderJSON(w, http.StatusCreated, c)
}

// accionCorte arma el handler de una acción sobre un corte (recalcular,
// anular o confirmar), que devuelve el corte como queda.
func (s *Servidor) accionCorte(fn func(ctx context.Context, u *acceso.Usuario, id int64) (cortes.CorteOptimizado, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := idDe(w, r)
		if !ok {
			return
		}
		c, err := fn(r.Context(), UsuarioDe(r.Context()), id)
		if err != nil {
			responderErrorCortes(w, r, err)
			return
		}
		responderJSON(w, http.StatusOK, c)
	}
}

func responderErrorCortes(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, cortes.ErrCorteNoEncontrado):
		responderError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, cortes.ErrCorteEstado):
		responderError(w, http.StatusConflict, err.Error())
	case errors.Is(err, stock.ErrStockInsuficiente):
		responderError(w, http.StatusUnprocessableEntity, err.Error())
	default:
		responderErrorStock(w, r, err)
	}
}
