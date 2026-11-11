package stock

import "context"

// Store guarda productos, categorías, movimientos de stock y retazos.
type Store interface {
	ListarRetazos(ctx context.Context, carpinteriaID int64, f FiltroRetazos) ([]RetazoStock, error)
	CrearRetazo(ctx context.Context, carpinteriaID int64, x RetazoStock) (int64, error)
	DescartarRetazo(ctx context.Context, carpinteriaID, id int64) error
	ListarProductos(ctx context.Context, carpinteriaID int64, f FiltroProductos) ([]ProductoConStock, error)
	BuscarProducto(ctx context.Context, carpinteriaID, id int64) (Producto, error)
	CrearProducto(ctx context.Context, carpinteriaID int64, p Producto) (int64, error)
	ActualizarProducto(ctx context.Context, carpinteriaID int64, p Producto) error
	ListarDepositos(ctx context.Context, carpinteriaID int64) ([]Deposito, error)
	// GuardarUbicaciones guarda dónde está el producto en cada depósito. Una
	// ubicación vacía borra la que había.
	GuardarUbicaciones(ctx context.Context, carpinteriaID, productoID int64, us []UbicacionNueva) error
	RegistrarOperaciones(ctx context.Context, carpinteriaID, usuarioID int64, ops []Operacion) error
	ListarMovimientos(ctx context.Context, carpinteriaID, productoID int64, limite int) ([]MovimientoRegistrado, error)
	ListarCategorias(ctx context.Context, carpinteriaID int64) ([]Categoria, error)
	CrearCategoria(ctx context.Context, carpinteriaID int64, c Categoria) (int64, error)
	ActualizarCategoria(ctx context.Context, carpinteriaID int64, c Categoria) error
	BorrarCategoria(ctx context.Context, carpinteriaID, id int64) error
}
