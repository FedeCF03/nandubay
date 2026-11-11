package cortes

import (
	"context"
	"errors"
	"net/http"

	"github.com/FedeCF03/nandubay/backend/internal/acceso"
	"github.com/FedeCF03/nandubay/backend/internal/stock"
	"github.com/FedeCF03/nandubay/backend/internal/web"
	"github.com/go-chi/chi/v5"
)

// Handler expone el módulo por HTTP: traduce pedidos a llamadas al Servicio
// y sus resultados a JSON. No tiene reglas de negocio.
type Handler struct {
	Servicio *Servicio
}

func (h *Handler) Rutas(r chi.Router) {
	r.With(acceso.RequierePermiso("cortes.generar")).Post("/cortes/optimizar", h.optimizarCortes)
	r.With(acceso.RequierePermiso("cortes.ver")).Get("/cortes", h.listarCortes)
	r.With(acceso.RequierePermiso("cortes.ver")).Get("/cortes/{id}", h.verCorte)
	r.With(acceso.RequierePermiso("cortes.generar")).Post("/cortes", h.crearCorte)
	r.With(acceso.RequierePermiso("cortes.generar")).Post("/cortes/{id}/recalcular", h.accionCorte(func(ctx context.Context, u *acceso.Usuario, id int64) (CorteOptimizado, error) {
		return h.Servicio.Recalcular(ctx, u, id)
	}))
	r.With(acceso.RequierePermiso("cortes.generar")).Post("/cortes/{id}/anular", h.accionCorte(func(ctx context.Context, u *acceso.Usuario, id int64) (CorteOptimizado, error) {
		return h.Servicio.AnularCorte(ctx, u, id)
	}))
	r.With(acceso.RequierePermiso("cortes.confirmar")).Post("/cortes/{id}/confirmar", h.accionCorte(func(ctx context.Context, u *acceso.Usuario, id int64) (CorteOptimizado, error) {
		return h.Servicio.ConfirmarCorte(ctx, u, id)
	}))
}

type pedidoOptimizar struct {
	Parametros   Parametros `json:"parametros"`
	Piezas       []Pieza    `json:"piezas"`
	MermaCantoMM int        `json:"merma_canto_mm"`
}

func (h *Handler) optimizarCortes(w http.ResponseWriter, r *http.Request) {
	var p pedidoOptimizar
	if !web.LeerJSON(w, r, &p) {
		return
	}
	if len(p.Piezas) == 0 {
		web.ResponderError(w, http.StatusBadRequest, "no hay piezas para cortar")
		return
	}
	plan, err := h.Servicio.Optimizar(p.Parametros, p.Piezas, p.MermaCantoMM)
	if err != nil {
		web.ResponderError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	web.ResponderJSON(w, http.StatusOK, plan)
}

func (h *Handler) listarCortes(w http.ResponseWriter, r *http.Request) {
	cs, err := h.Servicio.ListarCortes(r.Context(), acceso.UsuarioDe(r.Context()), r.URL.Query().Get("estado"))
	if err != nil {
		responderErrorCortes(w, r, err)
		return
	}
	web.ResponderJSON(w, http.StatusOK, cs)
}

func (h *Handler) verCorte(w http.ResponseWriter, r *http.Request) {
	id, ok := web.IDDe(w, r)
	if !ok {
		return
	}
	c, err := h.Servicio.VerCorte(r.Context(), acceso.UsuarioDe(r.Context()), id)
	if err != nil {
		responderErrorCortes(w, r, err)
		return
	}
	web.ResponderJSON(w, http.StatusOK, c)
}

func (h *Handler) crearCorte(w http.ResponseWriter, r *http.Request) {
	var n CorteNuevo
	if !web.LeerJSON(w, r, &n) {
		return
	}
	c, err := h.Servicio.CrearCorte(r.Context(), acceso.UsuarioDe(r.Context()), n)
	if err != nil {
		responderErrorCortes(w, r, err)
		return
	}
	web.ResponderJSON(w, http.StatusCreated, c)
}

// accionCorte arma el handler de una acción sobre un corte (recalcular,
// anular o confirmar), que devuelve el corte como queda.
func (h *Handler) accionCorte(fn func(ctx context.Context, u *acceso.Usuario, id int64) (CorteOptimizado, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := web.IDDe(w, r)
		if !ok {
			return
		}
		c, err := fn(r.Context(), acceso.UsuarioDe(r.Context()), id)
		if err != nil {
			responderErrorCortes(w, r, err)
			return
		}
		web.ResponderJSON(w, http.StatusOK, c)
	}
}

func responderErrorCortes(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrCorteNoEncontrado):
		web.ResponderError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrCorteEstado):
		web.ResponderError(w, http.StatusConflict, err.Error())
	case errors.Is(err, stock.ErrStockInsuficiente):
		web.ResponderError(w, http.StatusUnprocessableEntity, err.Error())
	default:
		stock.ResponderError(w, r, err)
	}
}
