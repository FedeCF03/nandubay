// Package service tiene la lógica de negocio de cada funcionalidad: valida,
// aplica las reglas de los modelos y guarda a través de un Store. Los Store son
// interfaces (Repository Pattern): la lógica no depende de Postgres, la
// implementación está en el paquete store y cmd conecta las dos cosas.
package service

import (
	"context"
	"time"

	"github.com/FedeCF03/nandubay/backend/internal/model/acceso"
	"github.com/FedeCF03/nandubay/backend/internal/model/cortes"
	"github.com/FedeCF03/nandubay/backend/internal/model/muebles"
	"github.com/FedeCF03/nandubay/backend/internal/model/stock"
	"github.com/FedeCF03/nandubay/backend/internal/model/ventas"
)

// StoreAcceso persiste cuentas, sesiones y carpinterías.
// Devuelve acceso.ErrNoEncontrado cuando no hay datos y acceso.ErrEmailEnUso
// cuando el email ya existe.
type StoreAcceso interface {
	BuscarCuenta(ctx context.Context, email string) (acceso.CuentaLogin, error)
	RegistrarIngreso(ctx context.Context, carpinteriaID, usuarioID int64) error
	CargarAcceso(ctx context.Context, carpinteriaID, usuarioID int64) (acceso.DatosAcceso, error)
	CrearCarpinteria(ctx context.Context, alta acceso.AltaCarpinteria) (int64, error)

	GuardarSesion(ctx context.Context, s acceso.Sesion) error
	// UsarSesion devuelve la sesión vigente y marca que se usó ahora.
	UsarSesion(ctx context.Context, tokenHash string, ahora time.Time) (acceso.Sesion, error)
	CambiarSucursalSesion(ctx context.Context, tokenHash string, sucursalID int64) error
	BorrarSesion(ctx context.Context, tokenHash string) error
	BorrarSesionesVencidas(ctx context.Context, ahora time.Time) error
}

// Hasher guarda y compara contraseñas.
type Hasher interface {
	Hash(clave string) (string, error)
	// Comparar con hash vacío tiene que tardar lo mismo y devolver false.
	Comparar(hash, clave string) bool
}

// Tokens genera tokens de sesión y calcula su hash.
type Tokens interface {
	Nuevo() (token, hash string, err error)
	Hash(token string) string
}

// Reloj permite fijar la hora en los tests.
type Reloj func() time.Time

// Interfaces de los stores. Los servicios dependen de estas interfaces y no de
// Postgres: la implementación está en el paquete store y se conecta en cmd.

// StoreStock guarda productos, categorías, movimientos de stock y retazos.
type StoreStock interface {
	ListarRetazos(ctx context.Context, carpinteriaID int64, f stock.FiltroRetazos) ([]stock.RetazoStock, error)
	CrearRetazo(ctx context.Context, carpinteriaID int64, x stock.RetazoStock) (int64, error)
	DescartarRetazo(ctx context.Context, carpinteriaID, id int64) error
	ListarProductos(ctx context.Context, carpinteriaID int64, f stock.FiltroProductos) ([]stock.ProductoConStock, error)
	BuscarProducto(ctx context.Context, carpinteriaID, id int64) (stock.Producto, error)
	CrearProducto(ctx context.Context, carpinteriaID int64, p stock.Producto) (int64, error)
	ActualizarProducto(ctx context.Context, carpinteriaID int64, p stock.Producto) error
	ListarDepositos(ctx context.Context, carpinteriaID int64) ([]stock.Deposito, error)
	// GuardarUbicaciones guarda dónde está el producto en cada depósito. Una
	// ubicación vacía borra la que había.
	GuardarUbicaciones(ctx context.Context, carpinteriaID, productoID int64, us []stock.UbicacionNueva) error
	RegistrarOperaciones(ctx context.Context, carpinteriaID, usuarioID int64, ops []stock.Operacion) error
	ListarMovimientos(ctx context.Context, carpinteriaID, productoID int64, limite int) ([]stock.MovimientoRegistrado, error)
	ListarCategorias(ctx context.Context, carpinteriaID int64) ([]stock.Categoria, error)
	CrearCategoria(ctx context.Context, carpinteriaID int64, c stock.Categoria) (int64, error)
	ActualizarCategoria(ctx context.Context, carpinteriaID int64, c stock.Categoria) error
	BorrarCategoria(ctx context.Context, carpinteriaID, id int64) error
}

// StoreVentas guarda clientes, ventas, pagos, retiros y presupuestos de muebles.
type StoreVentas interface {
	ListarFotos(ctx context.Context, carpinteriaID, pedidoID int64) ([]ventas.FotoMueble, error)
	AgregarFoto(ctx context.Context, carpinteriaID, usuarioID, pedidoID int64, f ventas.FotoNueva) (ventas.FotoMueble, error)
	BuscarFoto(ctx context.Context, carpinteriaID, pedidoID, fotoID int64) (tipo string, datos []byte, err error)
	BorrarFoto(ctx context.Context, carpinteriaID, pedidoID, fotoID int64) error
	RetazosDisponibles(ctx context.Context, carpinteriaID int64, placaIDs []int64, depositoID int64) (map[int64][]cortes.Retazo, error)
	BuscarMaterialesConfirmados(ctx context.Context, carpinteriaID, pedidoID int64) (*muebles.Materiales, error)
	ListarClientes(ctx context.Context, carpinteriaID int64, buscar string, limite int) ([]ventas.Cliente, error)
	CrearCliente(ctx context.Context, carpinteriaID int64, c ventas.Cliente) (int64, error)
	ActualizarCliente(ctx context.Context, carpinteriaID int64, c ventas.Cliente) error
	BuscarProductos(ctx context.Context, carpinteriaID int64, ids []int64) (map[int64]stock.Producto, error)
	ListarVentas(ctx context.Context, carpinteriaID int64, f ventas.FiltroVentas) ([]ventas.ResumenVenta, error)
	BuscarVenta(ctx context.Context, carpinteriaID, id int64) (ventas.Pedido, error)
	BuscarDespiece(ctx context.Context, carpinteriaID, pedidoID int64) ([]muebles.Mueble, error)
	CrearVenta(ctx context.Context, carpinteriaID int64, v ventas.VentaNueva) (int64, error)
	Modificar(ctx context.Context, carpinteriaID, usuarioID, id int64, fn func(ventas.Pedido) (ventas.Cambio, error)) error
	ProductosVendidos(ctx context.Context, carpinteriaID int64, f ventas.FiltroVendidos) ([]ventas.ProductoVendido, error)
}

// StoreCortes guarda los cortes optimizados.
type StoreCortes interface {
	BuscarProductos(ctx context.Context, carpinteriaID int64, ids []int64) (map[int64]stock.Producto, error)
	RetazosDisponibles(ctx context.Context, carpinteriaID int64, placaIDs []int64, depositoID int64) (map[int64][]cortes.Retazo, error)
	ListarCortes(ctx context.Context, carpinteriaID int64, estado string) ([]cortes.CorteOptimizado, error)
	BuscarCorte(ctx context.Context, carpinteriaID, id int64) (cortes.CorteOptimizado, error)
	CrearCorte(ctx context.Context, carpinteriaID, usuarioID int64, c cortes.CorteOptimizado) (int64, error)
	ActualizarResultado(ctx context.Context, carpinteriaID, id int64, res cortes.Resultado) error
	Anular(ctx context.Context, carpinteriaID, id int64) error
	// ConfirmarCorte registra que el corte se hizo: descuenta las placas enteras
	// del depósito, marca los retazos usados y guarda los sobrantes como retazos.
	ConfirmarCorte(ctx context.Context, carpinteriaID, usuarioID, id int64) error
}
