package stock

import (
	"context"
	"fmt"
	"strings"

	"github.com/FedeCF03/nandubay/backend/internal/acceso"
)

// Servicio implementa los casos de uso de productos y stock.
// Todos reciben el usuario que actúa: de ahí sale la carpintería y queda
// registrado quién hizo cada movimiento.
type Servicio struct {
	Store Store
}

func (s *Servicio) ListarProductos(ctx context.Context, u *acceso.Usuario, f FiltroProductos) ([]ProductoConStock, error) {
	ps, err := s.Store.ListarProductos(ctx, u.CarpinteriaID, f)
	if err != nil {
		return nil, err
	}
	for i := range ps {
		ps[i].Bajo = EstaBajo(ps[i].Disponible, ps[i].StockMinimo)
	}
	return ps, nil
}

func (s *Servicio) CrearProducto(ctx context.Context, u *acceso.Usuario, p Producto) (Producto, error) {
	p.Normalizar()
	p.Activo = true
	if err := p.Validar(); err != nil {
		return p, err
	}
	id, err := s.Store.CrearProducto(ctx, u.CarpinteriaID, p)
	p.ID = id
	return p, err
}

func (s *Servicio) ActualizarProducto(ctx context.Context, u *acceso.Usuario, p Producto) (Producto, error) {
	actual, err := s.Store.BuscarProducto(ctx, u.CarpinteriaID, p.ID)
	if err != nil {
		return p, err
	}
	// El tipo no se cambia: los movimientos ya registrados están en su unidad.
	p.Tipo = actual.Tipo
	p.Normalizar()
	if err := p.Validar(); err != nil {
		return p, err
	}
	return p, s.Store.ActualizarProducto(ctx, u.CarpinteriaID, p)
}

// GuardarUbicaciones guarda dónde está guardado un producto en cada depósito.
func (s *Servicio) GuardarUbicaciones(ctx context.Context, u *acceso.Usuario, productoID int64, us []UbicacionNueva) error {
	us, err := NormalizarUbicaciones(us)
	if err != nil {
		return err
	}
	return s.Store.GuardarUbicaciones(ctx, u.CarpinteriaID, productoID, us)
}

func (s *Servicio) ListarDepositos(ctx context.Context, u *acceso.Usuario) ([]Deposito, error) {
	return s.Store.ListarDepositos(ctx, u.CarpinteriaID)
}

func (s *Servicio) ListarMovimientos(ctx context.Context, u *acceso.Usuario, productoID int64) ([]MovimientoRegistrado, error) {
	if _, err := s.Store.BuscarProducto(ctx, u.CarpinteriaID, productoID); err != nil {
		return nil, err
	}
	return s.Store.ListarMovimientos(ctx, u.CarpinteriaID, productoID, 200)
}

func (s *Servicio) Ingresar(ctx context.Context, u *acceso.Usuario, productoID, depositoID int64, cantidad float64, costo *float64, nota string) error {
	op, err := Ingreso(productoID, depositoID, cantidad, costo, nota)
	if err != nil {
		return err
	}
	return s.registrar(ctx, u, productoID, []Operacion{op})
}

func (s *Servicio) Ajustar(ctx context.Context, u *acceso.Usuario, productoID, depositoID int64, contado float64, motivo string) error {
	op, err := Ajuste(productoID, depositoID, contado, motivo)
	if err != nil {
		return err
	}
	return s.registrar(ctx, u, productoID, []Operacion{op})
}

func (s *Servicio) Traspasar(ctx context.Context, u *acceso.Usuario, productoID, origen, destino int64, cantidad float64, nota string) error {
	ops, err := Traspaso(productoID, origen, destino, cantidad, nota)
	if err != nil {
		return err
	}
	return s.registrar(ctx, u, productoID, ops)
}

func (s *Servicio) registrar(ctx context.Context, u *acceso.Usuario, productoID int64, ops []Operacion) error {
	p, err := s.Store.BuscarProducto(ctx, u.CarpinteriaID, productoID)
	if err != nil {
		return err
	}
	if !p.Activo {
		return ErrValidacion{Campo: "producto", Mensaje: "el producto está dado de baja"}
	}
	return s.Store.RegistrarOperaciones(ctx, u.CarpinteriaID, u.ID, ops)
}

func (s *Servicio) ListarRetazos(ctx context.Context, u *acceso.Usuario, f FiltroRetazos) ([]RetazoStock, error) {
	return s.Store.ListarRetazos(ctx, u.CarpinteriaID, f)
}

// CrearRetazo guarda un retazo cargado a mano: tiene que ser de una placa y
// no puede ser más grande que la placa entera.
func (s *Servicio) CrearRetazo(ctx context.Context, u *acceso.Usuario, r RetazoStock) (RetazoStock, error) {
	p, err := s.Store.BuscarProducto(ctx, u.CarpinteriaID, r.PlacaID)
	if err != nil {
		return RetazoStock{}, err
	}
	if p.Tipo != TipoPlaca {
		return RetazoStock{}, ErrValidacion{Campo: "placa_id", Mensaje: "elegí una placa"}
	}
	if r.Largo <= 0 || r.Ancho <= 0 {
		return RetazoStock{}, ErrValidacion{Campo: "largo", Mensaje: "poné el largo y el ancho del retazo"}
	}
	if r.Largo > p.LargoMM || r.Ancho > p.AnchoMM {
		return RetazoStock{}, ErrValidacion{Campo: "largo",
			Mensaje: fmt.Sprintf("el retazo no puede ser más grande que la placa (%d × %d mm); el largo va en el sentido de la veta", p.LargoMM, p.AnchoMM)}
	}
	r.Nota = strings.TrimSpace(r.Nota)
	id, err := s.Store.CrearRetazo(ctx, u.CarpinteriaID, r)
	if err != nil {
		return RetazoStock{}, err
	}
	rs, err := s.Store.ListarRetazos(ctx, u.CarpinteriaID, FiltroRetazos{PlacaID: r.PlacaID, DepositoID: r.DepositoID})
	for _, x := range rs {
		if x.ID == id {
			return x, nil
		}
	}
	return RetazoStock{ID: id}, err
}

func (s *Servicio) DescartarRetazo(ctx context.Context, u *acceso.Usuario, id int64) error {
	return s.Store.DescartarRetazo(ctx, u.CarpinteriaID, id)
}

func (s *Servicio) ListarCategorias(ctx context.Context, u *acceso.Usuario) ([]Categoria, error) {
	return s.Store.ListarCategorias(ctx, u.CarpinteriaID)
}

func (s *Servicio) CrearCategoria(ctx context.Context, u *acceso.Usuario, c Categoria) (Categoria, error) {
	c.Normalizar()
	c.Productos = 0
	if err := c.Validar(); err != nil {
		return c, err
	}
	id, err := s.Store.CrearCategoria(ctx, u.CarpinteriaID, c)
	c.ID = id
	return c, err
}

func (s *Servicio) ActualizarCategoria(ctx context.Context, u *acceso.Usuario, c Categoria) (Categoria, error) {
	c.Normalizar()
	if err := c.Validar(); err != nil {
		return c, err
	}
	return c, s.Store.ActualizarCategoria(ctx, u.CarpinteriaID, c)
}

func (s *Servicio) BorrarCategoria(ctx context.Context, u *acceso.Usuario, id int64) error {
	return s.Store.BorrarCategoria(ctx, u.CarpinteriaID, id)
}
