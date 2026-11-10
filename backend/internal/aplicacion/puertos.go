// Package aplicacion tiene los casos de uso del sistema. Orquesta las reglas
// del dominio y habla con el mundo exterior solo a través de los puertos
// (interfaces) de este archivo, que implementan los adaptadores.
package aplicacion

import (
	"context"
	"time"

	"github.com/FedeCF03/nandubay/backend/internal/dominio/acceso"
	"github.com/FedeCF03/nandubay/backend/internal/dominio/stock"
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
