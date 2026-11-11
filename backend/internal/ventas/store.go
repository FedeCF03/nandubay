package ventas

import (
	"context"

	"github.com/FedeCF03/nandubay/backend/internal/cortes"
	"github.com/FedeCF03/nandubay/backend/internal/muebles"
	"github.com/FedeCF03/nandubay/backend/internal/stock"
)

// Store guarda clientes, ventas, pagos, retiros y presupuestos de muebles.
type Store interface {
	ListarFotos(ctx context.Context, carpinteriaID, pedidoID int64) ([]FotoMueble, error)
	AgregarFoto(ctx context.Context, carpinteriaID, usuarioID, pedidoID int64, f FotoNueva) (FotoMueble, error)
	BuscarFoto(ctx context.Context, carpinteriaID, pedidoID, fotoID int64) (tipo string, datos []byte, err error)
	BorrarFoto(ctx context.Context, carpinteriaID, pedidoID, fotoID int64) error
	RetazosDisponibles(ctx context.Context, carpinteriaID int64, placaIDs []int64, depositoID int64) (map[int64][]cortes.Retazo, error)
	BuscarMaterialesConfirmados(ctx context.Context, carpinteriaID, pedidoID int64) (*muebles.Materiales, error)
	ListarClientes(ctx context.Context, carpinteriaID int64, buscar string, limite int) ([]Cliente, error)
	CrearCliente(ctx context.Context, carpinteriaID int64, c Cliente) (int64, error)
	ActualizarCliente(ctx context.Context, carpinteriaID int64, c Cliente) error
	BuscarProductos(ctx context.Context, carpinteriaID int64, ids []int64) (map[int64]stock.Producto, error)
	ListarVentas(ctx context.Context, carpinteriaID int64, f FiltroVentas) ([]ResumenVenta, error)
	BuscarVenta(ctx context.Context, carpinteriaID, id int64) (Pedido, error)
	BuscarDespiece(ctx context.Context, carpinteriaID, pedidoID int64) ([]muebles.Mueble, error)
	CrearVenta(ctx context.Context, carpinteriaID int64, v VentaNueva) (int64, error)
	Modificar(ctx context.Context, carpinteriaID, usuarioID, id int64, fn func(Pedido) (Cambio, error)) error
	ProductosVendidos(ctx context.Context, carpinteriaID int64, f FiltroVendidos) ([]ProductoVendido, error)
}
