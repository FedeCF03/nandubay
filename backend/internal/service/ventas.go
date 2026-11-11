package service

import (
	"context"
	"time"

	"github.com/FedeCF03/nandubay/backend/internal/model/acceso"
	"github.com/FedeCF03/nandubay/backend/internal/model/ventas"
)

// ServicioVentas implementa los casos de uso de clientes, presupuestos,
// ventas, pagos y retiros.
type ServicioVentas struct {
	Store StoreVentas
}

// VentaVista es una venta con los importes calculados y lo que el usuario
// puede hacer con ella desde la sucursal en la que está.
type VentaVista struct {
	ventas.Pedido
	Total             float64         `json:"total"`
	Pagado            float64         `json:"pagado"`
	Saldo             float64         `json:"saldo"`
	Pagada            bool            `json:"pagada"`
	RetiraAqui        bool            `json:"retira_aqui"`
	TodoRetirado      bool            `json:"todo_retirado"`
	Carpinteria       string          `json:"carpinteria"`
	EstadosProduccion []ventas.Estado `json:"estados_produccion"`
}

func vista(u *acceso.Usuario, p ventas.Pedido) VentaVista {
	v := VentaVista{
		Pedido: p, Total: p.Total(), Pagado: p.Pagado(), Saldo: p.Saldo(),
		Pagada: p.EstaPagada(), TodoRetirado: p.TodoRetirado(), Carpinteria: u.Carpinteria,
		EstadosProduccion: ventas.EstadosProduccion,
	}
	v.RetiraAqui = p.Retira != nil && p.Retira.ID == u.SucursalID
	if v.Lineas == nil {
		v.Lineas = []ventas.Linea{}
	}
	if v.Pagos == nil {
		v.Pagos = []ventas.Pago{}
	}
	if v.Retiros == nil {
		v.Retiros = []ventas.Retiro{}
	}
	return v
}

func (s *ServicioVentas) ListarClientes(ctx context.Context, u *acceso.Usuario, buscar string) ([]ventas.Cliente, error) {
	return s.Store.ListarClientes(ctx, u.CarpinteriaID, buscar, 50)
}

func (s *ServicioVentas) CrearCliente(ctx context.Context, u *acceso.Usuario, c ventas.Cliente) (ventas.Cliente, error) {
	if err := c.Normalizar(); err != nil {
		return c, err
	}
	id, err := s.Store.CrearCliente(ctx, u.CarpinteriaID, c)
	c.ID = id
	return c, err
}

func (s *ServicioVentas) ActualizarCliente(ctx context.Context, u *acceso.Usuario, c ventas.Cliente) (ventas.Cliente, error) {
	if err := c.Normalizar(); err != nil {
		return c, err
	}
	return c, s.Store.ActualizarCliente(ctx, u.CarpinteriaID, c)
}

func (s *ServicioVentas) ListarVentas(ctx context.Context, u *acceso.Usuario, f ventas.FiltroVentas) ([]ventas.ResumenVenta, error) {
	if f.Limite <= 0 || f.Limite > 200 {
		f.Limite = 100
	}
	return s.Store.ListarVentas(ctx, u.CarpinteriaID, f)
}

func (s *ServicioVentas) VerVenta(ctx context.Context, u *acceso.Usuario, id int64) (VentaVista, error) {
	p, err := s.Store.BuscarVenta(ctx, u.CarpinteriaID, id)
	if err != nil {
		return VentaVista{}, err
	}
	return vista(u, p), nil
}

func (s *ServicioVentas) armarLineas(ctx context.Context, u *acceso.Usuario, nuevas []ventas.LineaNueva) ([]ventas.Linea, error) {
	var ids []int64
	for _, n := range nuevas {
		if n.Tipo == ventas.LineaProducto {
			ids = append(ids, n.ProductoID)
		}
	}
	productos, err := s.Store.BuscarProductos(ctx, u.CarpinteriaID, ids)
	if err != nil {
		return nil, err
	}
	return ventas.ArmarLineas(nuevas, productos)
}

// CrearPresupuesto guarda una venta nueva en la sucursal en la que está el usuario.
func (s *ServicioVentas) CrearPresupuesto(ctx context.Context, u *acceso.Usuario, tipo ventas.TipoVenta, d ventas.DatosVenta, nuevas []ventas.LineaNueva) (VentaVista, error) {
	lineas, err := s.armarLineas(ctx, u, nuevas)
	if err != nil {
		return VentaVista{}, err
	}
	if err := ventas.ControlarLineas(tipo, lineas); err != nil {
		return VentaVista{}, err
	}
	id, err := s.Store.CrearVenta(ctx, u.CarpinteriaID, ventas.VentaNueva{
		Tipo: tipo, SucursalID: u.SucursalID, VendedorID: u.ID, Datos: limpiarDatos(d), Lineas: lineas,
	})
	if err != nil {
		return VentaVista{}, err
	}
	return s.VerVenta(ctx, u, id)
}

func limpiarDatos(d ventas.DatosVenta) ventas.DatosVenta {
	if d.FechaEntrega != nil && *d.FechaEntrega == "" {
		d.FechaEntrega = nil
	}
	return d
}

// modificar aplica una operación del dominio y devuelve la venta actualizada.
func (s *ServicioVentas) modificar(ctx context.Context, u *acceso.Usuario, id int64, fn func(ventas.Pedido) (ventas.Cambio, error)) (VentaVista, error) {
	if err := s.Store.Modificar(ctx, u.CarpinteriaID, u.ID, id, fn); err != nil {
		return VentaVista{}, err
	}
	return s.VerVenta(ctx, u, id)
}

func (s *ServicioVentas) EditarPresupuesto(ctx context.Context, u *acceso.Usuario, id int64, d ventas.DatosVenta, nuevas []ventas.LineaNueva) (VentaVista, error) {
	lineas, err := s.armarLineas(ctx, u, nuevas)
	if err != nil {
		return VentaVista{}, err
	}
	d = limpiarDatos(d)
	return s.modificar(ctx, u, id, func(p ventas.Pedido) (ventas.Cambio, error) {
		if p.Tipo == ventas.Muebles {
			return ventas.Cambio{}, ventas.ErrValidacion{Campo: "lineas", Mensaje: "un presupuesto de muebles se edita desde presupuestos de muebles"}
		}
		return p.Editar(d, lineas)
	})
}

func (s *ServicioVentas) Confirmar(ctx context.Context, u *acceso.Usuario, id, depositoID int64) (VentaVista, error) {
	consumos, plan, err := s.materialesAConsumir(ctx, u, id, depositoID)
	if err != nil {
		return VentaVista{}, err
	}
	return s.modificar(ctx, u, id, func(p ventas.Pedido) (ventas.Cambio, error) {
		c, err := p.Confirmar(depositoID, consumos, time.Now())
		c.Materiales = plan
		return c, err
	})
}

func (s *ServicioVentas) RegistrarPago(ctx context.Context, u *acceso.Usuario, id int64, pago ventas.Pago) (VentaVista, error) {
	return s.modificar(ctx, u, id, func(p ventas.Pedido) (ventas.Cambio, error) { return p.RegistrarPago(pago, time.Now()) })
}

func (s *ServicioVentas) CambiarEstado(ctx context.Context, u *acceso.Usuario, id int64, e ventas.Estado) (VentaVista, error) {
	return s.modificar(ctx, u, id, func(p ventas.Pedido) (ventas.Cambio, error) { return p.CambiarEstado(e) })
}

// Retirar registra una entrega en la sucursal en la que está el usuario.
func (s *ServicioVentas) Retirar(ctx context.Context, u *acceso.Usuario, id int64, items []ventas.RetiroItem, nota string) (VentaVista, error) {
	return s.modificar(ctx, u, id, func(p ventas.Pedido) (ventas.Cambio, error) { return p.Retirar(u.SucursalID, items, nota) })
}

func (s *ServicioVentas) Cancelar(ctx context.Context, u *acceso.Usuario, id int64) (VentaVista, error) {
	return s.modificar(ctx, u, id, func(p ventas.Pedido) (ventas.Cambio, error) { return p.Cancelar() })
}

// ProductosVendidos resume qué productos del stock se vendieron en el período.
// Si no se dice, toma el mes en curso.
func (s *ServicioVentas) ProductosVendidos(ctx context.Context, u *acceso.Usuario, f ventas.FiltroVendidos) ([]ventas.ProductoVendido, error) {
	if f.Desde.IsZero() {
		hoy := time.Now()
		f.Desde = time.Date(hoy.Year(), hoy.Month(), 1, 0, 0, 0, 0, hoy.Location())
	}
	if f.Hasta.IsZero() || !f.Hasta.After(f.Desde) {
		f.Hasta = f.Desde.AddDate(0, 1, 0)
	}
	if f.Hasta.Sub(f.Desde) > 400*24*time.Hour {
		return nil, ventas.ErrValidacion{Campo: "hasta", Mensaje: "el período no puede ser de más de un año"}
	}
	return s.Store.ProductosVendidos(ctx, u.CarpinteriaID, f)
}
