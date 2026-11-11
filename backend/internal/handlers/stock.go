package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/FedeCF03/nandubay/backend/internal/modelos/acceso"
	"github.com/FedeCF03/nandubay/backend/internal/modelos/stock"
	"github.com/go-chi/chi/v5"
)

// CasosStock son los casos de uso de productos y stock.
type CasosStock interface {
	ListarProductos(ctx context.Context, u *acceso.Usuario, f stock.FiltroProductos) ([]stock.ProductoConStock, error)
	CrearProducto(ctx context.Context, u *acceso.Usuario, p stock.Producto) (stock.Producto, error)
	ActualizarProducto(ctx context.Context, u *acceso.Usuario, p stock.Producto) (stock.Producto, error)
	ListarDepositos(ctx context.Context, u *acceso.Usuario) ([]stock.Deposito, error)
	GuardarUbicaciones(ctx context.Context, u *acceso.Usuario, productoID int64, us []stock.UbicacionNueva) error
	ListarMovimientos(ctx context.Context, u *acceso.Usuario, productoID int64) ([]stock.MovimientoRegistrado, error)
	Ingresar(ctx context.Context, u *acceso.Usuario, productoID, depositoID int64, cantidad float64, costo *float64, nota string) error
	Ajustar(ctx context.Context, u *acceso.Usuario, productoID, depositoID int64, contado float64, motivo string) error
	Traspasar(ctx context.Context, u *acceso.Usuario, productoID, origen, destino int64, cantidad float64, nota string) error
	ListarRetazos(ctx context.Context, u *acceso.Usuario, f stock.FiltroRetazos) ([]stock.RetazoStock, error)
	CrearRetazo(ctx context.Context, u *acceso.Usuario, r stock.RetazoStock) (stock.RetazoStock, error)
	DescartarRetazo(ctx context.Context, u *acceso.Usuario, id int64) error
	ListarCategorias(ctx context.Context, u *acceso.Usuario) ([]stock.Categoria, error)
	CrearCategoria(ctx context.Context, u *acceso.Usuario, c stock.Categoria) (stock.Categoria, error)
	ActualizarCategoria(ctx context.Context, u *acceso.Usuario, c stock.Categoria) (stock.Categoria, error)
	BorrarCategoria(ctx context.Context, u *acceso.Usuario, id int64) error
}

func (s *Servidor) rutasStock(r chi.Router) {
	r.With(requierePermiso("productos.ver")).Get("/productos", s.listarProductos)
	r.With(requierePermiso("productos.editar")).Post("/productos", s.crearProducto)
	r.With(requierePermiso("productos.editar")).Put("/productos/{id}", s.actualizarProducto)
	r.With(requierePermiso("stock.ver")).Get("/productos/{id}/movimientos", s.listarMovimientos)
	r.With(requierePermiso("stock.ver")).Get("/depositos", s.listarDepositos)
	r.With(requierePermiso("stock.editar")).Put("/productos/{id}/ubicaciones", s.guardarUbicaciones)
	r.With(requierePermiso("stock.editar")).Post("/stock/ingresos", s.ingresar)
	r.With(requierePermiso("stock.editar")).Post("/stock/ajustes", s.ajustar)
	r.With(requierePermiso("stock.editar")).Post("/stock/traspasos", s.traspasar)
	r.With(requierePermiso("stock.ver")).Get("/retazos", s.listarRetazos)
	r.With(requierePermiso("stock.editar")).Post("/retazos", s.crearRetazo)
	r.With(requierePermiso("stock.editar")).Post("/retazos/{id}/descartar", s.descartarRetazo)
	r.With(requierePermiso("productos.ver")).Get("/categorias", s.listarCategorias)
	r.With(requierePermiso("productos.editar")).Post("/categorias", s.crearCategoria)
	r.With(requierePermiso("productos.editar")).Put("/categorias/{id}", s.actualizarCategoria)
	r.With(requierePermiso("productos.editar")).Delete("/categorias/{id}", s.borrarCategoria)
}

func (s *Servidor) guardarUbicaciones(w http.ResponseWriter, r *http.Request) {
	id, ok := idDe(w, r)
	if !ok {
		return
	}
	var in struct {
		Ubicaciones []stock.UbicacionNueva `json:"ubicaciones"`
	}
	if !leerJSON(w, r, &in) {
		return
	}
	if err := s.Stock.GuardarUbicaciones(r.Context(), UsuarioDe(r.Context()), id, in.Ubicaciones); err != nil {
		responderErrorStock(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Servidor) listarRetazos(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	placa, _ := strconv.ParseInt(q.Get("placa_id"), 10, 64)
	deposito, _ := strconv.ParseInt(q.Get("deposito_id"), 10, 64)
	rs, err := s.Stock.ListarRetazos(r.Context(), UsuarioDe(r.Context()), stock.FiltroRetazos{PlacaID: placa, DepositoID: deposito})
	if err != nil {
		responderErrorStock(w, r, err)
		return
	}
	responderJSON(w, http.StatusOK, rs)
}

func (s *Servidor) crearRetazo(w http.ResponseWriter, r *http.Request) {
	var in stock.RetazoStock
	if !leerJSON(w, r, &in) {
		return
	}
	x, err := s.Stock.CrearRetazo(r.Context(), UsuarioDe(r.Context()), in)
	if err != nil {
		responderErrorStock(w, r, err)
		return
	}
	responderJSON(w, http.StatusCreated, x)
}

func (s *Servidor) descartarRetazo(w http.ResponseWriter, r *http.Request) {
	id, ok := idDe(w, r)
	if !ok {
		return
	}
	if err := s.Stock.DescartarRetazo(r.Context(), UsuarioDe(r.Context()), id); err != nil {
		responderErrorStock(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// responderErrorStock traduce los errores del dominio a códigos HTTP.
func responderErrorStock(w http.ResponseWriter, r *http.Request, err error) {
	var val stock.ErrValidacion
	switch {
	case errors.As(err, &val):
		responderJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": val.Mensaje, "campo": val.Campo})
	case errors.Is(err, stock.ErrNoEncontrado):
		responderError(w, http.StatusNotFound, "no se encontró el producto, el depósito o la categoría")
	case errors.Is(err, stock.ErrStockInsuficiente), errors.Is(err, stock.ErrMismoDeposito),
		errors.Is(err, stock.ErrCantidad):
		responderError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, stock.ErrSkuEnUso), errors.Is(err, stock.ErrCategoriaEnUso):
		responderError(w, http.StatusConflict, err.Error())
	default:
		errorInterno(w, r, err)
	}
}

func idDe(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		responderError(w, http.StatusBadRequest, "id inválido")
		return 0, false
	}
	return id, true
}

func (s *Servidor) listarProductos(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ps, err := s.Stock.ListarProductos(r.Context(), UsuarioDe(r.Context()), stock.FiltroProductos{
		Tipo: q.Get("tipo"), Buscar: q.Get("buscar"), Inactivos: q.Get("inactivos") == "1",
		CategoriaID: categoriaDeFiltro(q.Get("categoria")),
	})
	if err != nil {
		responderErrorStock(w, r, err)
		return
	}
	if ps == nil {
		ps = []stock.ProductoConStock{}
	}
	responderJSON(w, http.StatusOK, ps)
}

func (s *Servidor) crearProducto(w http.ResponseWriter, r *http.Request) {
	var p stock.Producto
	if !leerJSON(w, r, &p) {
		return
	}
	p, err := s.Stock.CrearProducto(r.Context(), UsuarioDe(r.Context()), p)
	if err != nil {
		responderErrorStock(w, r, err)
		return
	}
	responderJSON(w, http.StatusCreated, p)
}

func (s *Servidor) actualizarProducto(w http.ResponseWriter, r *http.Request) {
	id, ok := idDe(w, r)
	if !ok {
		return
	}
	var p stock.Producto
	if !leerJSON(w, r, &p) {
		return
	}
	p.ID = id
	p, err := s.Stock.ActualizarProducto(r.Context(), UsuarioDe(r.Context()), p)
	if err != nil {
		responderErrorStock(w, r, err)
		return
	}
	responderJSON(w, http.StatusOK, p)
}

func (s *Servidor) listarMovimientos(w http.ResponseWriter, r *http.Request) {
	id, ok := idDe(w, r)
	if !ok {
		return
	}
	ms, err := s.Stock.ListarMovimientos(r.Context(), UsuarioDe(r.Context()), id)
	if err != nil {
		responderErrorStock(w, r, err)
		return
	}
	if ms == nil {
		ms = []stock.MovimientoRegistrado{}
	}
	responderJSON(w, http.StatusOK, ms)
}

func (s *Servidor) listarDepositos(w http.ResponseWriter, r *http.Request) {
	ds, err := s.Stock.ListarDepositos(r.Context(), UsuarioDe(r.Context()))
	if err != nil {
		responderErrorStock(w, r, err)
		return
	}
	responderJSON(w, http.StatusOK, ds)
}

type pedidoMovimiento struct {
	ProductoID int64    `json:"producto_id"`
	DepositoID int64    `json:"deposito_id"`
	DestinoID  int64    `json:"destino_id"`
	Cantidad   float64  `json:"cantidad"`
	Costo      *float64 `json:"costo_unitario"`
	Nota       string   `json:"nota"`
}

func (s *Servidor) movimiento(w http.ResponseWriter, r *http.Request, fn func(p pedidoMovimiento) error) {
	var p pedidoMovimiento
	if !leerJSON(w, r, &p) {
		return
	}
	if err := fn(p); err != nil {
		responderErrorStock(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Servidor) ingresar(w http.ResponseWriter, r *http.Request) {
	s.movimiento(w, r, func(p pedidoMovimiento) error {
		return s.Stock.Ingresar(r.Context(), UsuarioDe(r.Context()), p.ProductoID, p.DepositoID, p.Cantidad, p.Costo, p.Nota)
	})
}

func (s *Servidor) ajustar(w http.ResponseWriter, r *http.Request) {
	s.movimiento(w, r, func(p pedidoMovimiento) error {
		return s.Stock.Ajustar(r.Context(), UsuarioDe(r.Context()), p.ProductoID, p.DepositoID, p.Cantidad, p.Nota)
	})
}

func (s *Servidor) traspasar(w http.ResponseWriter, r *http.Request) {
	s.movimiento(w, r, func(p pedidoMovimiento) error {
		return s.Stock.Traspasar(r.Context(), UsuarioDe(r.Context()), p.ProductoID, p.DepositoID, p.DestinoID, p.Cantidad, p.Nota)
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

func (s *Servidor) listarCategorias(w http.ResponseWriter, r *http.Request) {
	cs, err := s.Stock.ListarCategorias(r.Context(), UsuarioDe(r.Context()))
	if err != nil {
		responderErrorStock(w, r, err)
		return
	}
	responderJSON(w, http.StatusOK, cs)
}

func (s *Servidor) crearCategoria(w http.ResponseWriter, r *http.Request) {
	var c stock.Categoria
	if !leerJSON(w, r, &c) {
		return
	}
	c, err := s.Stock.CrearCategoria(r.Context(), UsuarioDe(r.Context()), c)
	if err != nil {
		responderErrorStock(w, r, err)
		return
	}
	responderJSON(w, http.StatusCreated, c)
}

func (s *Servidor) actualizarCategoria(w http.ResponseWriter, r *http.Request) {
	id, ok := idDe(w, r)
	if !ok {
		return
	}
	var c stock.Categoria
	if !leerJSON(w, r, &c) {
		return
	}
	c.ID = id
	c, err := s.Stock.ActualizarCategoria(r.Context(), UsuarioDe(r.Context()), c)
	if err != nil {
		responderErrorStock(w, r, err)
		return
	}
	responderJSON(w, http.StatusOK, c)
}

func (s *Servidor) borrarCategoria(w http.ResponseWriter, r *http.Request) {
	id, ok := idDe(w, r)
	if !ok {
		return
	}
	if err := s.Stock.BorrarCategoria(r.Context(), UsuarioDe(r.Context()), id); err != nil {
		responderErrorStock(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
