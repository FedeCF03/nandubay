package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/FedeCF03/nandubay/backend/internal/aplicacion"
	"github.com/FedeCF03/nandubay/backend/internal/dominio/acceso"
	"github.com/FedeCF03/nandubay/backend/internal/dominio/ventas"
)

// CasosVentas son los casos de uso de clientes y ventas.
type CasosVentas interface {
	ListarClientes(ctx context.Context, u *acceso.Usuario, buscar string) ([]ventas.Cliente, error)
	CrearCliente(ctx context.Context, u *acceso.Usuario, c ventas.Cliente) (ventas.Cliente, error)
	ActualizarCliente(ctx context.Context, u *acceso.Usuario, c ventas.Cliente) (ventas.Cliente, error)
	ListarVentas(ctx context.Context, u *acceso.Usuario, f aplicacion.FiltroVentas) ([]aplicacion.ResumenVenta, error)
	VerVenta(ctx context.Context, u *acceso.Usuario, id int64) (aplicacion.VentaVista, error)
	CrearPresupuesto(ctx context.Context, u *acceso.Usuario, tipo ventas.TipoVenta, d ventas.DatosVenta, ls []ventas.LineaNueva) (aplicacion.VentaVista, error)
	EditarPresupuesto(ctx context.Context, u *acceso.Usuario, id int64, d ventas.DatosVenta, ls []ventas.LineaNueva) (aplicacion.VentaVista, error)
	Confirmar(ctx context.Context, u *acceso.Usuario, id, depositoID int64) (aplicacion.VentaVista, error)
	RegistrarPago(ctx context.Context, u *acceso.Usuario, id int64, p ventas.Pago) (aplicacion.VentaVista, error)
	CambiarEstado(ctx context.Context, u *acceso.Usuario, id int64, e ventas.Estado) (aplicacion.VentaVista, error)
	Retirar(ctx context.Context, u *acceso.Usuario, id int64, items []ventas.RetiroItem, nota string) (aplicacion.VentaVista, error)
	Cancelar(ctx context.Context, u *acceso.Usuario, id int64) (aplicacion.VentaVista, error)
	ProductosVendidos(ctx context.Context, u *acceso.Usuario, f aplicacion.FiltroVendidos) ([]aplicacion.ProductoVendido, error)
}

func (s *Servidor) rutasVentas(r chi.Router) {
	r.With(requierePermiso("clientes.ver")).Get("/clientes", s.listarClientes)
	r.With(requierePermiso("clientes.editar")).Post("/clientes", s.crearCliente)
	r.With(requierePermiso("clientes.editar")).Put("/clientes/{id}", s.actualizarCliente)

	r.With(requierePermiso("pedidos.ver")).Get("/ventas", s.listarVentas)
	r.With(requierePermiso("pedidos.ver")).Get("/ventas/productos-vendidos", s.productosVendidos)
	r.With(requierePermiso("pedidos.ver")).Get("/ventas/{id}", s.verVenta)
	r.With(requierePermiso("pedidos.editar")).Post("/ventas", s.crearPresupuesto)
	r.With(requierePermiso("pedidos.editar")).Put("/ventas/{id}", s.editarPresupuesto)
	r.With(requierePermiso("pedidos.confirmar")).Post("/ventas/{id}/confirmar", s.confirmarVenta)
	r.With(requierePermiso("pedidos.confirmar")).Post("/ventas/{id}/cancelar", s.cancelarVenta)
	r.With(requierePermiso("pedidos.estado")).Post("/ventas/{id}/estado", s.cambiarEstado)
	r.With(requierePermiso("pagos.registrar")).Post("/ventas/{id}/pagos", s.registrarPago)
	r.With(requierePermiso("retiros.registrar")).Post("/ventas/{id}/retiros", s.registrarRetiro)
}

// responderErrorVentas traduce los errores del dominio a códigos HTTP.
func responderErrorVentas(w http.ResponseWriter, r *http.Request, err error) {
	var val ventas.ErrValidacion
	switch {
	case errors.As(err, &val):
		responderJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": val.Mensaje, "campo": val.Campo})
	case errors.Is(err, ventas.ErrNoEncontrado):
		responderError(w, http.StatusNotFound, "no se encontró la venta o el cliente")
	case errors.Is(err, ventas.ErrEstado), errors.Is(err, ventas.ErrConPagos), errors.Is(err, ventas.ErrConRetiros):
		responderError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ventas.ErrSinLineas), errors.Is(err, ventas.ErrSaldoPendiente),
		errors.Is(err, ventas.ErrOtraSucursal), errors.Is(err, ventas.ErrPagoExcedido),
		errors.Is(err, ventas.ErrNadaQueRetirar), errors.Is(err, ventas.ErrMuebleNoListo):
		responderError(w, http.StatusUnprocessableEntity, err.Error())
	default:
		// Errores del stock al confirmar o cancelar.
		responderErrorStock(w, r, err)
	}
}

func (s *Servidor) listarClientes(w http.ResponseWriter, r *http.Request) {
	cs, err := s.Ventas.ListarClientes(r.Context(), UsuarioDe(r.Context()), r.URL.Query().Get("buscar"))
	if err != nil {
		responderErrorVentas(w, r, err)
		return
	}
	responderJSON(w, http.StatusOK, cs)
}

func (s *Servidor) crearCliente(w http.ResponseWriter, r *http.Request) {
	var c ventas.Cliente
	if !leerJSON(w, r, &c) {
		return
	}
	c, err := s.Ventas.CrearCliente(r.Context(), UsuarioDe(r.Context()), c)
	if err != nil {
		responderErrorVentas(w, r, err)
		return
	}
	responderJSON(w, http.StatusCreated, c)
}

func (s *Servidor) actualizarCliente(w http.ResponseWriter, r *http.Request) {
	id, ok := idDe(w, r)
	var c ventas.Cliente
	if !ok || !leerJSON(w, r, &c) {
		return
	}
	c.ID = id
	c, err := s.Ventas.ActualizarCliente(r.Context(), UsuarioDe(r.Context()), c)
	if err != nil {
		responderErrorVentas(w, r, err)
		return
	}
	responderJSON(w, http.StatusOK, c)
}

func (s *Servidor) listarVentas(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limite, _ := strconv.Atoi(q.Get("limite"))
	vs, err := s.Ventas.ListarVentas(r.Context(), UsuarioDe(r.Context()), aplicacion.FiltroVentas{
		Tipo: ventas.TipoVenta(q.Get("tipo")), Estado: q.Get("estado"), Buscar: q.Get("buscar"), Limite: limite,
	})
	if err != nil {
		responderErrorVentas(w, r, err)
		return
	}
	responderJSON(w, http.StatusOK, vs)
}

func (s *Servidor) verVenta(w http.ResponseWriter, r *http.Request) {
	id, ok := idDe(w, r)
	if !ok {
		return
	}
	s.responderVenta(w, r, http.StatusOK)(s.Ventas.VerVenta(r.Context(), UsuarioDe(r.Context()), id))
}

// responderVenta devuelve la venta actualizada o el error.
func (s *Servidor) responderVenta(w http.ResponseWriter, r *http.Request, status int) func(aplicacion.VentaVista, error) {
	return func(v aplicacion.VentaVista, err error) {
		if err != nil {
			responderErrorVentas(w, r, err)
			return
		}
		responderJSON(w, status, v)
	}
}

type pedidoVenta struct {
	ventas.DatosVenta
	Lineas []ventas.LineaNueva `json:"lineas"`
}

func (s *Servidor) crearPresupuesto(w http.ResponseWriter, r *http.Request) {
	var in pedidoVenta
	if !leerJSON(w, r, &in) {
		return
	}
	s.responderVenta(w, r, http.StatusCreated)(s.Ventas.CrearPresupuesto(r.Context(), UsuarioDe(r.Context()), ventas.Mostrador, in.DatosVenta, in.Lineas))
}

func (s *Servidor) editarPresupuesto(w http.ResponseWriter, r *http.Request) {
	id, ok := idDe(w, r)
	var in pedidoVenta
	if !ok || !leerJSON(w, r, &in) {
		return
	}
	s.responderVenta(w, r, http.StatusOK)(s.Ventas.EditarPresupuesto(r.Context(), UsuarioDe(r.Context()), id, in.DatosVenta, in.Lineas))
}

func (s *Servidor) confirmarVenta(w http.ResponseWriter, r *http.Request) {
	id, ok := idDe(w, r)
	var in struct {
		DepositoID int64 `json:"deposito_id"`
	}
	if !ok || !leerJSON(w, r, &in) {
		return
	}
	s.responderVenta(w, r, http.StatusOK)(s.Ventas.Confirmar(r.Context(), UsuarioDe(r.Context()), id, in.DepositoID))
}

func (s *Servidor) cancelarVenta(w http.ResponseWriter, r *http.Request) {
	id, ok := idDe(w, r)
	if !ok {
		return
	}
	s.responderVenta(w, r, http.StatusOK)(s.Ventas.Cancelar(r.Context(), UsuarioDe(r.Context()), id))
}

func (s *Servidor) cambiarEstado(w http.ResponseWriter, r *http.Request) {
	id, ok := idDe(w, r)
	var in struct {
		Estado ventas.Estado `json:"estado"`
	}
	if !ok || !leerJSON(w, r, &in) {
		return
	}
	s.responderVenta(w, r, http.StatusOK)(s.Ventas.CambiarEstado(r.Context(), UsuarioDe(r.Context()), id, in.Estado))
}

func (s *Servidor) registrarPago(w http.ResponseWriter, r *http.Request) {
	id, ok := idDe(w, r)
	var in struct {
		Tipo  ventas.TipoPago `json:"tipo"`
		Monto float64         `json:"monto"`
		Medio string          `json:"medio"`
		Nota  string          `json:"nota"`
	}
	if !ok || !leerJSON(w, r, &in) {
		return
	}
	p := ventas.Pago{Tipo: in.Tipo, Monto: in.Monto, Medio: in.Medio, Nota: in.Nota}
	s.responderVenta(w, r, http.StatusOK)(s.Ventas.RegistrarPago(r.Context(), UsuarioDe(r.Context()), id, p))
}

func (s *Servidor) registrarRetiro(w http.ResponseWriter, r *http.Request) {
	id, ok := idDe(w, r)
	var in struct {
		Items []ventas.RetiroItem `json:"items"`
		Nota  string              `json:"nota"`
	}
	if !ok || !leerJSON(w, r, &in) {
		return
	}
	s.responderVenta(w, r, http.StatusOK)(s.Ventas.Retirar(r.Context(), UsuarioDe(r.Context()), id, in.Items, in.Nota))
}

func (s *Servidor) productosVendidos(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var f aplicacion.FiltroVendidos
	var err error
	// Las fechas vienen como AAAA-MM-DD; "hasta" es el último día incluido.
	if v := q.Get("desde"); v != "" {
		if f.Desde, err = time.ParseInLocation("2006-01-02", v, zonaHoraria); err != nil {
			responderError(w, http.StatusBadRequest, "fecha desde inválida")
			return
		}
	}
	if v := q.Get("hasta"); v != "" {
		var h time.Time
		if h, err = time.ParseInLocation("2006-01-02", v, zonaHoraria); err != nil {
			responderError(w, http.StatusBadRequest, "fecha hasta inválida")
			return
		}
		f.Hasta = h.AddDate(0, 0, 1)
	}
	f.SucursalID, _ = strconv.ParseInt(q.Get("sucursal_id"), 10, 64)
	vs, err := s.Ventas.ProductosVendidos(r.Context(), UsuarioDe(r.Context()), f)
	if err != nil {
		responderErrorVentas(w, r, err)
		return
	}
	responderJSON(w, http.StatusOK, vs)
}

// zonaHoraria es la de Argentina: un día de ventas va de 0 a 24 hs de acá,
// aunque el servidor esté en UTC.
var zonaHoraria = func() *time.Location {
	if z, err := time.LoadLocation("America/Argentina/Buenos_Aires"); err == nil {
		return z
	}
	return time.Local
}()
