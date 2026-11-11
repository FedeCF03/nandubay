package stock

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/FedeCF03/nandubay/backend/internal/acceso"
	"github.com/FedeCF03/nandubay/backend/internal/web"
	"github.com/go-chi/chi/v5"
)

// Handler expone el módulo por HTTP: traduce pedidos a llamadas al Servicio
// y sus resultados a JSON. No tiene reglas de negocio.
type Handler struct {
	Servicio *Servicio
}

func (h *Handler) Rutas(r chi.Router) {
	r.With(acceso.RequierePermiso("productos.ver")).Get("/productos", h.listarProductos)
	r.With(acceso.RequierePermiso("productos.editar")).Post("/productos", h.crearProducto)
	r.With(acceso.RequierePermiso("productos.editar")).Put("/productos/{id}", h.actualizarProducto)
	r.With(acceso.RequierePermiso("stock.ver")).Get("/productos/{id}/movimientos", h.listarMovimientos)
	r.With(acceso.RequierePermiso("stock.ver")).Get("/depositos", h.listarDepositos)
	r.With(acceso.RequierePermiso("stock.editar")).Put("/productos/{id}/ubicaciones", h.guardarUbicaciones)
	r.With(acceso.RequierePermiso("stock.editar")).Post("/stock/ingresos", h.ingresar)
	r.With(acceso.RequierePermiso("stock.editar")).Post("/stock/ajustes", h.ajustar)
	r.With(acceso.RequierePermiso("stock.editar")).Post("/stock/traspasos", h.traspasar)
	r.With(acceso.RequierePermiso("stock.ver")).Get("/retazos", h.listarRetazos)
	r.With(acceso.RequierePermiso("stock.editar")).Post("/retazos", h.crearRetazo)
	r.With(acceso.RequierePermiso("stock.editar")).Post("/retazos/{id}/descartar", h.descartarRetazo)
	r.With(acceso.RequierePermiso("productos.ver")).Get("/categorias", h.listarCategorias)
	r.With(acceso.RequierePermiso("productos.editar")).Post("/categorias", h.crearCategoria)
	r.With(acceso.RequierePermiso("productos.editar")).Put("/categorias/{id}", h.actualizarCategoria)
	r.With(acceso.RequierePermiso("productos.editar")).Delete("/categorias/{id}", h.borrarCategoria)
}

func (h *Handler) guardarUbicaciones(w http.ResponseWriter, r *http.Request) {
	id, ok := web.IDDe(w, r)
	if !ok {
		return
	}
	var in struct {
		Ubicaciones []UbicacionNueva `json:"ubicaciones"`
	}
	if !web.LeerJSON(w, r, &in) {
		return
	}
	if err := h.Servicio.GuardarUbicaciones(r.Context(), acceso.UsuarioDe(r.Context()), id, in.Ubicaciones); err != nil {
		ResponderError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listarRetazos(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	placa, _ := strconv.ParseInt(q.Get("placa_id"), 10, 64)
	deposito, _ := strconv.ParseInt(q.Get("deposito_id"), 10, 64)
	rs, err := h.Servicio.ListarRetazos(r.Context(), acceso.UsuarioDe(r.Context()), FiltroRetazos{PlacaID: placa, DepositoID: deposito})
	if err != nil {
		ResponderError(w, r, err)
		return
	}
	web.ResponderJSON(w, http.StatusOK, rs)
}

func (h *Handler) crearRetazo(w http.ResponseWriter, r *http.Request) {
	var in RetazoStock
	if !web.LeerJSON(w, r, &in) {
		return
	}
	x, err := h.Servicio.CrearRetazo(r.Context(), acceso.UsuarioDe(r.Context()), in)
	if err != nil {
		ResponderError(w, r, err)
		return
	}
	web.ResponderJSON(w, http.StatusCreated, x)
}

func (h *Handler) descartarRetazo(w http.ResponseWriter, r *http.Request) {
	id, ok := web.IDDe(w, r)
	if !ok {
		return
	}
	if err := h.Servicio.DescartarRetazo(r.Context(), acceso.UsuarioDe(r.Context()), id); err != nil {
		ResponderError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ResponderError traduce los errores de stock a códigos HTTP. Lo usan también
// los otros módulos cuando el error viene del stock.
func ResponderError(w http.ResponseWriter, r *http.Request, err error) {
	var val ErrValidacion
	switch {
	case errors.As(err, &val):
		web.ResponderJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": val.Mensaje, "campo": val.Campo})
	case errors.Is(err, ErrNoEncontrado):
		web.ResponderError(w, http.StatusNotFound, "no se encontró el producto, el depósito o la categoría")
	case errors.Is(err, ErrStockInsuficiente), errors.Is(err, ErrMismoDeposito),
		errors.Is(err, ErrCantidad):
		web.ResponderError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, ErrSkuEnUso), errors.Is(err, ErrCategoriaEnUso):
		web.ResponderError(w, http.StatusConflict, err.Error())
	default:
		web.ErrorInterno(w, r, err)
	}
}

func (h *Handler) listarProductos(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ps, err := h.Servicio.ListarProductos(r.Context(), acceso.UsuarioDe(r.Context()), FiltroProductos{
		Tipo: q.Get("tipo"), Buscar: q.Get("buscar"), Inactivos: q.Get("inactivos") == "1",
		CategoriaID: categoriaDeFiltro(q.Get("categoria")),
	})
	if err != nil {
		ResponderError(w, r, err)
		return
	}
	if ps == nil {
		ps = []ProductoConStock{}
	}
	web.ResponderJSON(w, http.StatusOK, ps)
}

func (h *Handler) crearProducto(w http.ResponseWriter, r *http.Request) {
	var p Producto
	if !web.LeerJSON(w, r, &p) {
		return
	}
	p, err := h.Servicio.CrearProducto(r.Context(), acceso.UsuarioDe(r.Context()), p)
	if err != nil {
		ResponderError(w, r, err)
		return
	}
	web.ResponderJSON(w, http.StatusCreated, p)
}

func (h *Handler) actualizarProducto(w http.ResponseWriter, r *http.Request) {
	id, ok := web.IDDe(w, r)
	if !ok {
		return
	}
	var p Producto
	if !web.LeerJSON(w, r, &p) {
		return
	}
	p.ID = id
	p, err := h.Servicio.ActualizarProducto(r.Context(), acceso.UsuarioDe(r.Context()), p)
	if err != nil {
		ResponderError(w, r, err)
		return
	}
	web.ResponderJSON(w, http.StatusOK, p)
}

func (h *Handler) listarMovimientos(w http.ResponseWriter, r *http.Request) {
	id, ok := web.IDDe(w, r)
	if !ok {
		return
	}
	ms, err := h.Servicio.ListarMovimientos(r.Context(), acceso.UsuarioDe(r.Context()), id)
	if err != nil {
		ResponderError(w, r, err)
		return
	}
	if ms == nil {
		ms = []MovimientoRegistrado{}
	}
	web.ResponderJSON(w, http.StatusOK, ms)
}

func (h *Handler) listarDepositos(w http.ResponseWriter, r *http.Request) {
	ds, err := h.Servicio.ListarDepositos(r.Context(), acceso.UsuarioDe(r.Context()))
	if err != nil {
		ResponderError(w, r, err)
		return
	}
	web.ResponderJSON(w, http.StatusOK, ds)
}

type pedidoMovimiento struct {
	ProductoID int64    `json:"producto_id"`
	DepositoID int64    `json:"deposito_id"`
	DestinoID  int64    `json:"destino_id"`
	Cantidad   float64  `json:"cantidad"`
	Costo      *float64 `json:"costo_unitario"`
	Nota       string   `json:"nota"`
}

func (h *Handler) movimiento(w http.ResponseWriter, r *http.Request, fn func(p pedidoMovimiento) error) {
	var p pedidoMovimiento
	if !web.LeerJSON(w, r, &p) {
		return
	}
	if err := fn(p); err != nil {
		ResponderError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ingresar(w http.ResponseWriter, r *http.Request) {
	h.movimiento(w, r, func(p pedidoMovimiento) error {
		return h.Servicio.Ingresar(r.Context(), acceso.UsuarioDe(r.Context()), p.ProductoID, p.DepositoID, p.Cantidad, p.Costo, p.Nota)
	})
}

func (h *Handler) ajustar(w http.ResponseWriter, r *http.Request) {
	h.movimiento(w, r, func(p pedidoMovimiento) error {
		return h.Servicio.Ajustar(r.Context(), acceso.UsuarioDe(r.Context()), p.ProductoID, p.DepositoID, p.Cantidad, p.Nota)
	})
}

func (h *Handler) traspasar(w http.ResponseWriter, r *http.Request) {
	h.movimiento(w, r, func(p pedidoMovimiento) error {
		return h.Servicio.Traspasar(r.Context(), acceso.UsuarioDe(r.Context()), p.ProductoID, p.DepositoID, p.DestinoID, p.Cantidad, p.Nota)
	})
}

// categoriaDeFiltro lee ?categoria=: un id, "sin" para los productos sin
// categoría, o vacío para todos.
func categoriaDeFiltro(v string) int64 {
	if v == "sin" {
		return -1
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}

func (h *Handler) listarCategorias(w http.ResponseWriter, r *http.Request) {
	cs, err := h.Servicio.ListarCategorias(r.Context(), acceso.UsuarioDe(r.Context()))
	if err != nil {
		ResponderError(w, r, err)
		return
	}
	web.ResponderJSON(w, http.StatusOK, cs)
}

func (h *Handler) crearCategoria(w http.ResponseWriter, r *http.Request) {
	var c Categoria
	if !web.LeerJSON(w, r, &c) {
		return
	}
	c, err := h.Servicio.CrearCategoria(r.Context(), acceso.UsuarioDe(r.Context()), c)
	if err != nil {
		ResponderError(w, r, err)
		return
	}
	web.ResponderJSON(w, http.StatusCreated, c)
}

func (h *Handler) actualizarCategoria(w http.ResponseWriter, r *http.Request) {
	id, ok := web.IDDe(w, r)
	if !ok {
		return
	}
	var c Categoria
	if !web.LeerJSON(w, r, &c) {
		return
	}
	c.ID = id
	c, err := h.Servicio.ActualizarCategoria(r.Context(), acceso.UsuarioDe(r.Context()), c)
	if err != nil {
		ResponderError(w, r, err)
		return
	}
	web.ResponderJSON(w, http.StatusOK, c)
}

func (h *Handler) borrarCategoria(w http.ResponseWriter, r *http.Request) {
	id, ok := web.IDDe(w, r)
	if !ok {
		return
	}
	if err := h.Servicio.BorrarCategoria(r.Context(), acceso.UsuarioDe(r.Context()), id); err != nil {
		ResponderError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
