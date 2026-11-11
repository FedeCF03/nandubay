package ventas

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/FedeCF03/nandubay/backend/internal/acceso"
	"github.com/FedeCF03/nandubay/backend/internal/muebles"
	"github.com/FedeCF03/nandubay/backend/internal/stock"
	"github.com/FedeCF03/nandubay/backend/internal/web"
)

// Handler expone el módulo por HTTP: traduce pedidos a llamadas al Servicio
// y sus resultados a JSON. No tiene reglas de negocio.
type Handler struct {
	Servicio *Servicio
}

func (h *Handler) Rutas(r chi.Router) {
	r.With(acceso.RequierePermiso("clientes.ver")).Get("/clientes", h.listarClientes)
	r.With(acceso.RequierePermiso("clientes.editar")).Post("/clientes", h.crearCliente)
	r.With(acceso.RequierePermiso("clientes.editar")).Put("/clientes/{id}", h.actualizarCliente)

	r.With(acceso.RequierePermiso("pedidos.ver")).Get("/ventas", h.listarVentas)
	r.With(acceso.RequierePermiso("pedidos.ver")).Get("/ventas/productos-vendidos", h.productosVendidos)
	r.With(acceso.RequierePermiso("pedidos.ver")).Get("/ventas/{id}", h.verVenta)
	r.With(acceso.RequierePermiso("pedidos.editar")).Post("/ventas", h.crearPresupuesto)
	r.With(acceso.RequierePermiso("pedidos.editar")).Put("/ventas/{id}", h.editarPresupuesto)
	r.With(acceso.RequierePermiso("pedidos.confirmar")).Post("/ventas/{id}/confirmar", h.confirmarVenta)
	r.With(acceso.RequierePermiso("pedidos.confirmar")).Post("/ventas/{id}/cancelar", h.cancelarVenta)
	r.With(acceso.RequierePermiso("pedidos.estado")).Post("/ventas/{id}/estado", h.cambiarEstado)
	r.With(acceso.RequierePermiso("pagos.registrar")).Post("/ventas/{id}/pagos", h.registrarPago)
	r.With(acceso.RequierePermiso("retiros.registrar")).Post("/ventas/{id}/retiros", h.registrarRetiro)

	r.With(acceso.RequierePermiso("pedidos.editar")).Post("/presupuestos/calcular", h.calcularMateriales)
	r.With(acceso.RequierePermiso("pedidos.editar")).Post("/presupuestos", h.crearPresupuestoMuebles)
	r.With(acceso.RequierePermiso("pedidos.ver")).Get("/presupuestos/{id}", h.verPresupuestoMuebles)
	r.With(acceso.RequierePermiso("pedidos.editar")).Put("/presupuestos/{id}", h.editarPresupuestoMuebles)
	r.With(acceso.RequierePermiso("pedidos.editar")).Post("/presupuestos/{id}/fotos", h.agregarFoto)
	r.With(acceso.RequierePermiso("pedidos.ver")).Get("/presupuestos/{id}/fotos/{foto}", h.verFoto)
	r.With(acceso.RequierePermiso("pedidos.editar")).Delete("/presupuestos/{id}/fotos/{foto}", h.borrarFoto)
}

// responderErrorVentas traduce los errores de ventas a códigos HTTP.
func responderErrorVentas(w http.ResponseWriter, r *http.Request, err error) {
	var val ErrValidacion
	var valMueble muebles.ErrValidacion
	switch {
	case errors.As(err, &val):
		web.ResponderJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": val.Mensaje, "campo": val.Campo})
	case errors.As(err, &valMueble):
		web.ResponderJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": valMueble.Mensaje, "campo": valMueble.Campo})
	case errors.Is(err, ErrNoEncontrado):
		web.ResponderError(w, http.StatusNotFound, "no se encontró la venta o el cliente")
	case errors.Is(err, ErrFotoInvalida):
		web.ResponderError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, ErrDemasiadasFotos):
		web.ResponderError(w, http.StatusConflict, fmt.Sprintf("cada mueble puede tener hasta %d fotos", MaxFotosPorMueble))
	case errors.Is(err, ErrEstado), errors.Is(err, ErrConPagos), errors.Is(err, ErrConRetiros):
		web.ResponderError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrSinLineas), errors.Is(err, ErrSaldoPendiente),
		errors.Is(err, ErrOtraSucursal), errors.Is(err, ErrPagoExcedido),
		errors.Is(err, ErrNadaQueRetirar), errors.Is(err, ErrMuebleNoListo):
		web.ResponderError(w, http.StatusUnprocessableEntity, err.Error())
	default:
		// Errores del stock al confirmar o cancelar.
		stock.ResponderError(w, r, err)
	}
}

func (h *Handler) listarClientes(w http.ResponseWriter, r *http.Request) {
	cs, err := h.Servicio.ListarClientes(r.Context(), acceso.UsuarioDe(r.Context()), r.URL.Query().Get("buscar"))
	if err != nil {
		responderErrorVentas(w, r, err)
		return
	}
	web.ResponderJSON(w, http.StatusOK, cs)
}

func (h *Handler) crearCliente(w http.ResponseWriter, r *http.Request) {
	var c Cliente
	if !web.LeerJSON(w, r, &c) {
		return
	}
	c, err := h.Servicio.CrearCliente(r.Context(), acceso.UsuarioDe(r.Context()), c)
	if err != nil {
		responderErrorVentas(w, r, err)
		return
	}
	web.ResponderJSON(w, http.StatusCreated, c)
}

func (h *Handler) actualizarCliente(w http.ResponseWriter, r *http.Request) {
	id, ok := web.IDDe(w, r)
	var c Cliente
	if !ok || !web.LeerJSON(w, r, &c) {
		return
	}
	c.ID = id
	c, err := h.Servicio.ActualizarCliente(r.Context(), acceso.UsuarioDe(r.Context()), c)
	if err != nil {
		responderErrorVentas(w, r, err)
		return
	}
	web.ResponderJSON(w, http.StatusOK, c)
}

func (h *Handler) listarVentas(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limite, _ := strconv.Atoi(q.Get("limite"))
	vs, err := h.Servicio.ListarVentas(r.Context(), acceso.UsuarioDe(r.Context()), FiltroVentas{
		Tipo: TipoVenta(q.Get("tipo")), Estado: q.Get("estado"), Buscar: q.Get("buscar"), Limite: limite,
	})
	if err != nil {
		responderErrorVentas(w, r, err)
		return
	}
	web.ResponderJSON(w, http.StatusOK, vs)
}

func (h *Handler) verVenta(w http.ResponseWriter, r *http.Request) {
	id, ok := web.IDDe(w, r)
	if !ok {
		return
	}
	h.responderVenta(w, r, http.StatusOK)(h.Servicio.VerVenta(r.Context(), acceso.UsuarioDe(r.Context()), id))
}

// responderVenta devuelve la venta actualizada o el error.
func (h *Handler) responderVenta(w http.ResponseWriter, r *http.Request, status int) func(VentaVista, error) {
	return func(v VentaVista, err error) {
		if err != nil {
			responderErrorVentas(w, r, err)
			return
		}
		web.ResponderJSON(w, status, v)
	}
}

type pedidoVenta struct {
	DatosVenta
	Lineas []LineaNueva `json:"lineas"`
}

func (h *Handler) crearPresupuesto(w http.ResponseWriter, r *http.Request) {
	var in pedidoVenta
	if !web.LeerJSON(w, r, &in) {
		return
	}
	h.responderVenta(w, r, http.StatusCreated)(h.Servicio.CrearPresupuesto(r.Context(), acceso.UsuarioDe(r.Context()), Mostrador, in.DatosVenta, in.Lineas))
}

func (h *Handler) editarPresupuesto(w http.ResponseWriter, r *http.Request) {
	id, ok := web.IDDe(w, r)
	var in pedidoVenta
	if !ok || !web.LeerJSON(w, r, &in) {
		return
	}
	h.responderVenta(w, r, http.StatusOK)(h.Servicio.EditarPresupuesto(r.Context(), acceso.UsuarioDe(r.Context()), id, in.DatosVenta, in.Lineas))
}

func (h *Handler) confirmarVenta(w http.ResponseWriter, r *http.Request) {
	id, ok := web.IDDe(w, r)
	var in struct {
		DepositoID int64 `json:"deposito_id"`
	}
	if !ok || !web.LeerJSON(w, r, &in) {
		return
	}
	h.responderVenta(w, r, http.StatusOK)(h.Servicio.Confirmar(r.Context(), acceso.UsuarioDe(r.Context()), id, in.DepositoID))
}

func (h *Handler) cancelarVenta(w http.ResponseWriter, r *http.Request) {
	id, ok := web.IDDe(w, r)
	if !ok {
		return
	}
	h.responderVenta(w, r, http.StatusOK)(h.Servicio.Cancelar(r.Context(), acceso.UsuarioDe(r.Context()), id))
}

func (h *Handler) cambiarEstado(w http.ResponseWriter, r *http.Request) {
	id, ok := web.IDDe(w, r)
	var in struct {
		Estado Estado `json:"estado"`
	}
	if !ok || !web.LeerJSON(w, r, &in) {
		return
	}
	h.responderVenta(w, r, http.StatusOK)(h.Servicio.CambiarEstado(r.Context(), acceso.UsuarioDe(r.Context()), id, in.Estado))
}

func (h *Handler) registrarPago(w http.ResponseWriter, r *http.Request) {
	id, ok := web.IDDe(w, r)
	var in struct {
		Tipo  TipoPago `json:"tipo"`
		Monto float64  `json:"monto"`
		Medio string   `json:"medio"`
		Nota  string   `json:"nota"`
	}
	if !ok || !web.LeerJSON(w, r, &in) {
		return
	}
	p := Pago{Tipo: in.Tipo, Monto: in.Monto, Medio: in.Medio, Nota: in.Nota}
	h.responderVenta(w, r, http.StatusOK)(h.Servicio.RegistrarPago(r.Context(), acceso.UsuarioDe(r.Context()), id, p))
}

func (h *Handler) registrarRetiro(w http.ResponseWriter, r *http.Request) {
	id, ok := web.IDDe(w, r)
	var in struct {
		Items []RetiroItem `json:"items"`
		Nota  string       `json:"nota"`
	}
	if !ok || !web.LeerJSON(w, r, &in) {
		return
	}
	h.responderVenta(w, r, http.StatusOK)(h.Servicio.Retirar(r.Context(), acceso.UsuarioDe(r.Context()), id, in.Items, in.Nota))
}

func (h *Handler) productosVendidos(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var f FiltroVendidos
	var err error
	// Las fechas vienen como AAAA-MM-DD; "hasta" es el último día incluido.
	if v := q.Get("desde"); v != "" {
		if f.Desde, err = time.ParseInLocation("2006-01-02", v, ZonaHoraria); err != nil {
			web.ResponderError(w, http.StatusBadRequest, "fecha desde inválida")
			return
		}
	}
	if v := q.Get("hasta"); v != "" {
		var h time.Time
		if h, err = time.ParseInLocation("2006-01-02", v, ZonaHoraria); err != nil {
			web.ResponderError(w, http.StatusBadRequest, "fecha hasta inválida")
			return
		}
		f.Hasta = h.AddDate(0, 0, 1)
	}
	f.SucursalID, _ = strconv.ParseInt(q.Get("sucursal_id"), 10, 64)
	vs, err := h.Servicio.ProductosVendidos(r.Context(), acceso.UsuarioDe(r.Context()), f)
	if err != nil {
		responderErrorVentas(w, r, err)
		return
	}
	web.ResponderJSON(w, http.StatusOK, vs)
}

// ZonaHoraria es la de Argentina: un día de ventas va de 0 a 24 hs de acá,
// aunque el servidor esté en UTC.
var ZonaHoraria = func() *time.Location {
	if z, err := time.LoadLocation("America/Argentina/Buenos_Aires"); err == nil {
		return z
	}
	return time.Local
}()

type presupuestoMuebles struct {
	DatosVenta
	Muebles []muebles.Mueble `json:"muebles"`
}

func (h *Handler) responderPresupuesto(w http.ResponseWriter, r *http.Request, status int) func(PresupuestoMuebles, error) {
	return func(p PresupuestoMuebles, err error) {
		if err != nil {
			responderErrorVentas(w, r, err)
			return
		}
		web.ResponderJSON(w, status, p)
	}
}

func (h *Handler) calcularMateriales(w http.ResponseWriter, r *http.Request) {
	var in presupuestoMuebles
	if !web.LeerJSON(w, r, &in) {
		return
	}
	mat, err := h.Servicio.CalcularMateriales(r.Context(), acceso.UsuarioDe(r.Context()), in.Muebles)
	if err != nil {
		responderErrorVentas(w, r, err)
		return
	}
	web.ResponderJSON(w, http.StatusOK, mat)
}

func (h *Handler) crearPresupuestoMuebles(w http.ResponseWriter, r *http.Request) {
	var in presupuestoMuebles
	if !web.LeerJSON(w, r, &in) {
		return
	}
	h.responderPresupuesto(w, r, http.StatusCreated)(h.Servicio.CrearPresupuestoMuebles(r.Context(), acceso.UsuarioDe(r.Context()), in.DatosVenta, in.Muebles))
}

func (h *Handler) editarPresupuestoMuebles(w http.ResponseWriter, r *http.Request) {
	id, ok := web.IDDe(w, r)
	var in presupuestoMuebles
	if !ok || !web.LeerJSON(w, r, &in) {
		return
	}
	h.responderPresupuesto(w, r, http.StatusOK)(h.Servicio.EditarPresupuestoMuebles(r.Context(), acceso.UsuarioDe(r.Context()), id, in.DatosVenta, in.Muebles))
}

func (h *Handler) verPresupuestoMuebles(w http.ResponseWriter, r *http.Request) {
	id, ok := web.IDDe(w, r)
	if !ok {
		return
	}
	h.responderPresupuesto(w, r, http.StatusOK)(h.Servicio.VerPresupuestoMuebles(r.Context(), acceso.UsuarioDe(r.Context()), id))
}

func (h *Handler) agregarFoto(w http.ResponseWriter, r *http.Request) {
	id, ok := web.IDDe(w, r)
	if !ok {
		return
	}
	// La imagen viaja en base64 dentro del JSON (así sigue valiendo la
	// protección CSRF de soloJSON); por eso el límite es mayor que el común.
	var in struct {
		MuebleClave string `json:"mueble_clave"`
		Datos       []byte `json:"datos"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxBytesFoto*4/3+4096)).Decode(&in); err != nil {
		web.ResponderError(w, http.StatusUnprocessableEntity, ErrFotoInvalida.Error())
		return
	}
	f, err := h.Servicio.AgregarFoto(r.Context(), acceso.UsuarioDe(r.Context()), id, in.MuebleClave, in.Datos)
	if err != nil {
		responderErrorVentas(w, r, err)
		return
	}
	web.ResponderJSON(w, http.StatusCreated, f)
}

func idFoto(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	id, ok := web.IDDe(w, r)
	if !ok {
		return 0, 0, false
	}
	foto, err := strconv.ParseInt(chi.URLParam(r, "foto"), 10, 64)
	if err != nil || foto <= 0 {
		web.ResponderError(w, http.StatusBadRequest, "id inválido")
		return 0, 0, false
	}
	return id, foto, true
}

func (h *Handler) verFoto(w http.ResponseWriter, r *http.Request) {
	id, foto, ok := idFoto(w, r)
	if !ok {
		return
	}
	tipo, datos, err := h.Servicio.VerFoto(r.Context(), acceso.UsuarioDe(r.Context()), id, foto)
	if err != nil {
		responderErrorVentas(w, r, err)
		return
	}
	cab := w.Header()
	cab.Set("Content-Type", tipo)
	cab.Set("X-Content-Type-Options", "nosniff")
	cab.Set("Content-Security-Policy", "default-src 'none'")
	// Una foto no cambia nunca (si se borra, la nueva tiene otro id).
	cab.Set("Cache-Control", "private, max-age=31536000, immutable")
	_, _ = w.Write(datos)
}

func (h *Handler) borrarFoto(w http.ResponseWriter, r *http.Request) {
	id, foto, ok := idFoto(w, r)
	if !ok {
		return
	}
	if err := h.Servicio.BorrarFoto(r.Context(), acceso.UsuarioDe(r.Context()), id, foto); err != nil {
		responderErrorVentas(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
