package acceso

import "time"

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
	SucursalesActivas []Sucursal
	Asignaciones      []Asignacion
}

// AltaCarpinteria es lo que se persiste al dar de alta una carpintería.
type AltaCarpinteria struct {
	Nombre         string
	Sucursal       string
	Deposito       string
	Roles          []RolBase
	RolAdmin       string
	AdminNombre    string
	AdminEmail     string
	AdminHashClave string
}
