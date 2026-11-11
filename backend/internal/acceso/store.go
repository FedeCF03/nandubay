package acceso

import (
	"context"
	"time"
)

// Store persiste cuentas, sesiones y carpinterías.
// Devuelve ErrNoEncontrado cuando no hay datos y ErrEmailEnUso
// cuando el email ya existe.
type Store interface {
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
