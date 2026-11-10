// Package aplicacion tiene los casos de uso del sistema. Orquesta las reglas
// del dominio y habla con el mundo exterior solo a través de los puertos
// (interfaces) de este archivo, que implementan los adaptadores.
package aplicacion

import (
	"context"
	"time"

	"github.com/FedeCF03/nandubay/backend/internal/dominio/acceso"
	"github.com/FedeCF03/nandubay/backend/internal/dominio/muebles"
	"github.com/FedeCF03/nandubay/backend/internal/dominio/stock"
	"github.com/FedeCF03/nandubay/backend/internal/dominio/ventas"
)

// CuentaLogin son los datos necesarios para validar un login.
type CuentaLogin struct {
	UsuarioID     int64
	CarpinteriaID int64
	HashClave     string
	Activo        bool
}

// Sesion abierta. Se guarda el hash del token, nunca el token.
type Sesion struct {
	TokenHash     string
	UsuarioID     int64
	CarpinteriaID int64
	SucursalID    *int64
	ExpiraEn      time.Time
	IP            string
	Agente        string
}

// DatosAcceso es lo que hace falta para resolver los permisos de un usuario.
type DatosAcceso struct {
	Nombre            string
	Email             string
	Activo            bool
	Carpinteria       string
	SucursalesActivas []acceso.Sucursal
	Asignaciones      []acceso.Asignacion
}

// AltaCarpinteria es lo que se persiste al dar de alta una carpintería.
type AltaCarpinteria struct {
	Nombre         string
	Sucursal       string
	Deposito       string
	Roles          []acceso.RolBase
	RolAdmin       string
	AdminNombre    string
	AdminEmail     string
	AdminHashClave string
}

// RepositorioAcceso persiste cuentas, sesiones y carpinterías.
// Devuelve acceso.ErrNoEncontrado cuando no hay datos y acceso.ErrEmailEnUso
// cuando el email ya existe.
type RepositorioAcceso interface {
	BuscarCuenta(ctx context.Context, email string) (CuentaLogin, error)
	RegistrarIngreso(ctx context.Context, carpinteriaID, usuarioID int64) error
	CargarAcceso(ctx context.Context, carpinteriaID, usuarioID int64) (DatosAcceso, error)
	CrearCarpinteria(ctx context.Context, alta AltaCarpinteria) (int64, error)

	GuardarSesion(ctx context.Context, s Sesion) error
	// UsarSesion devuelve la sesión vigente y marca que se usó ahora.
	UsarSesion(ctx context.Context, tokenHash string, ahora time.Time) (Sesion, error)
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

// FiltroProductos para listar productos.
type FiltroProductos struct {
	Tipo      string
	Buscar    string
	Inactivos bool
}

// ProductoConStock es un producto con su stock en cada depósito.
type ProductoConStock struct {
	stock.Producto
	PorDeposito map[int64]float64 `json:"por_deposito"`
	Total       float64           `json:"total"`
	Bajo        bool              `json:"bajo"`
}

// MovimientoRegistrado es un movimiento ya guardado, para el historial.
type MovimientoRegistrado struct {
	ID int64 `json:"id"`
	stock.Movimiento
	Deposito string    `json:"deposito"`
	Usuario  string    `json:"usuario"`
	Fecha    time.Time `json:"fecha"`
}

// RepositorioStock persiste productos y movimientos de una carpintería.
// Devuelve stock.ErrNoEncontrado y stock.ErrSkuEnUso cuando corresponde.
type RepositorioStock interface {
	ListarProductos(ctx context.Context, carpinteriaID int64, f FiltroProductos) ([]ProductoConStock, error)
	BuscarProducto(ctx context.Context, carpinteriaID, id int64) (stock.Producto, error)
	CrearProducto(ctx context.Context, carpinteriaID int64, p stock.Producto) (int64, error)
	ActualizarProducto(ctx context.Context, carpinteriaID int64, p stock.Producto) error
	ListarDepositos(ctx context.Context, carpinteriaID int64) ([]stock.Deposito, error)
	// RegistrarOperaciones aplica todas las operaciones en una sola transacción,
	// con cada saldo bloqueado mientras se calcula el nuevo. Si una falla, no se aplica ninguna.
	RegistrarOperaciones(ctx context.Context, carpinteriaID, usuarioID int64, ops []stock.Operacion) error
	ListarMovimientos(ctx context.Context, carpinteriaID, productoID int64, limite int) ([]MovimientoRegistrado, error)
}

// FiltroVentas para listar ventas.
type FiltroVentas struct {
	Tipo   ventas.TipoVenta // vacío = todas
	Estado string           // vacío = todas; "a_retirar" = confirmadas con algo pendiente
	Buscar string           // número de venta o nombre del cliente
	Limite int
}

// ResumenVenta es un renglón del listado de ventas.
type ResumenVenta struct {
	ID       int64         `json:"id"`
	Numero   int           `json:"numero"`
	Estado   ventas.Estado `json:"estado"`
	Cliente  string        `json:"cliente"`
	Sucursal string        `json:"sucursal"`
	Retira   string        `json:"retira"`
	Total    float64       `json:"total"`
	Saldo    float64       `json:"saldo"`
	Fecha    time.Time     `json:"fecha"`
}

// VentaNueva es un presupuesto a guardar.
type VentaNueva struct {
	Tipo       ventas.TipoVenta
	SucursalID int64
	VendedorID int64
	Datos      ventas.DatosVenta
	Lineas     []ventas.Linea
	Despiece   []muebles.Mueble // en un presupuesto de muebles, uno por línea
}

// RepositorioVentas persiste clientes y ventas. Devuelve ventas.ErrNoEncontrado
// si la venta, el cliente o el depósito no son de la carpintería.
type RepositorioVentas interface {
	ListarClientes(ctx context.Context, carpinteriaID int64, buscar string, limite int) ([]ventas.Cliente, error)
	CrearCliente(ctx context.Context, carpinteriaID int64, c ventas.Cliente) (int64, error)
	ActualizarCliente(ctx context.Context, carpinteriaID int64, c ventas.Cliente) error
	// BuscarProductos trae los productos activos con esos ids.
	BuscarProductos(ctx context.Context, carpinteriaID int64, ids []int64) (map[int64]stock.Producto, error)
	ListarVentas(ctx context.Context, carpinteriaID int64, f FiltroVentas) ([]ResumenVenta, error)
	BuscarVenta(ctx context.Context, carpinteriaID, id int64) (ventas.Pedido, error)
	CrearVenta(ctx context.Context, carpinteriaID int64, v VentaNueva) (int64, error)
	// Modificar lee la venta bloqueada, le pide a fn el cambio y lo guarda en
	// la misma transacción, junto con los movimientos de stock. Si algo
	// falla no se guarda nada.
	Modificar(ctx context.Context, carpinteriaID, usuarioID, id int64, fn func(ventas.Pedido) (ventas.Cambio, error)) error
	ProductosVendidos(ctx context.Context, carpinteriaID int64, f FiltroVendidos) ([]ProductoVendido, error)
	// BuscarDespiece trae los muebles de un presupuesto con sus piezas e insumos.
	BuscarDespiece(ctx context.Context, carpinteriaID, pedidoID int64) ([]muebles.Mueble, error)
}

// FiltroVendidos limita el resumen de productos vendidos. Cuenta las ventas
// confirmadas entre Desde (incluido) y Hasta (excluido), por fecha de confirmación.
type FiltroVendidos struct {
	Desde, Hasta time.Time
	SucursalID   int64 // 0 = todas
}

// ProductoVendido es cuánto se vendió de un producto en el período.
type ProductoVendido struct {
	ProductoID int64   `json:"producto_id"`
	Nombre     string  `json:"nombre"`
	Tipo       string  `json:"tipo"`
	Unidad     string  `json:"unidad"`
	Cantidad   float64 `json:"cantidad"`
	Importe    float64 `json:"importe"`
	Costo      float64 `json:"costo"`
	Ventas     int     `json:"ventas"`
}
