// Package servicios tiene la lógica de cada funcionalidad: valida, aplica las
// reglas de los modelos y guarda con los repositorios. Usa los repositorios
// directamente; solo el de acceso, el hash y los tokens son interfaces, para
// poder probar el login sin base de datos.
package servicios

import (
	"context"
	"time"

	"github.com/FedeCF03/nandubay/backend/internal/modelos/acceso"
)

// RepositorioAcceso persiste cuentas, sesiones y carpinterías.
// Devuelve acceso.ErrNoEncontrado cuando no hay datos y acceso.ErrEmailEnUso
// cuando el email ya existe.
type RepositorioAcceso interface {
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
