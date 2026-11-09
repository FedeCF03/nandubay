package aplicacion

import (
	"context"

	"github.com/FedeCF03/nandubay/backend/internal/dominio/acceso"
	"github.com/FedeCF03/nandubay/backend/internal/dominio/stock"
)

// ServicioStock implementa los casos de uso de productos y stock.
// Todos reciben el usuario que actúa: de ahí sale la carpintería y queda
// registrado quién hizo cada movimiento.
type ServicioStock struct {
	Repo RepositorioStock
}

func (s *ServicioStock) ListarProductos(ctx context.Context, u *acceso.Usuario, f FiltroProductos) ([]ProductoConStock, error) {
	ps, err := s.Repo.ListarProductos(ctx, u.CarpinteriaID, f)
	if err != nil {
		return nil, err
	}
	for i := range ps {
		ps[i].Bajo = stock.EstaBajo(ps[i].Total, ps[i].StockMinimo)
	}
	return ps, nil
}

func (s *ServicioStock) CrearProducto(ctx context.Context, u *acceso.Usuario, p stock.Producto) (stock.Producto, error) {
	p.Normalizar()
	p.Activo = true
	if err := p.Validar(); err != nil {
		return p, err
	}
	id, err := s.Repo.CrearProducto(ctx, u.CarpinteriaID, p)
	p.ID = id
	return p, err
}

func (s *ServicioStock) ActualizarProducto(ctx context.Context, u *acceso.Usuario, p stock.Producto) (stock.Producto, error) {
	actual, err := s.Repo.BuscarProducto(ctx, u.CarpinteriaID, p.ID)
	if err != nil {
		return p, err
	}
	// El tipo no se cambia: los movimientos ya registrados están en su unidad.
	p.Tipo = actual.Tipo
	p.Normalizar()
	if err := p.Validar(); err != nil {
		return p, err
	}
	return p, s.Repo.ActualizarProducto(ctx, u.CarpinteriaID, p)
}

func (s *ServicioStock) ListarDepositos(ctx context.Context, u *acceso.Usuario) ([]stock.Deposito, error) {
	return s.Repo.ListarDepositos(ctx, u.CarpinteriaID)
}

func (s *ServicioStock) ListarMovimientos(ctx context.Context, u *acceso.Usuario, productoID int64) ([]MovimientoRegistrado, error) {
	if _, err := s.Repo.BuscarProducto(ctx, u.CarpinteriaID, productoID); err != nil {
		return nil, err
	}
	return s.Repo.ListarMovimientos(ctx, u.CarpinteriaID, productoID, 200)
}

func (s *ServicioStock) Ingresar(ctx context.Context, u *acceso.Usuario, productoID, depositoID int64, cantidad float64, costo *float64, nota string) error {
	op, err := stock.Ingreso(productoID, depositoID, cantidad, costo, nota)
	if err != nil {
		return err
	}
	return s.registrar(ctx, u, productoID, []stock.Operacion{op})
}

func (s *ServicioStock) Ajustar(ctx context.Context, u *acceso.Usuario, productoID, depositoID int64, contado float64, motivo string) error {
	op, err := stock.Ajuste(productoID, depositoID, contado, motivo)
	if err != nil {
		return err
	}
	return s.registrar(ctx, u, productoID, []stock.Operacion{op})
}

func (s *ServicioStock) Traspasar(ctx context.Context, u *acceso.Usuario, productoID, origen, destino int64, cantidad float64, nota string) error {
	ops, err := stock.Traspaso(productoID, origen, destino, cantidad, nota)
	if err != nil {
		return err
	}
	return s.registrar(ctx, u, productoID, ops)
}

func (s *ServicioStock) registrar(ctx context.Context, u *acceso.Usuario, productoID int64, ops []stock.Operacion) error {
	p, err := s.Repo.BuscarProducto(ctx, u.CarpinteriaID, productoID)
	if err != nil {
		return err
	}
	if !p.Activo {
		return stock.ErrValidacion{Campo: "producto", Mensaje: "el producto está dado de baja"}
	}
	return s.Repo.RegistrarOperaciones(ctx, u.CarpinteriaID, u.ID, ops)
}
