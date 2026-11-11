// Package acceso tiene las entidades y reglas de usuarios, sucursales, roles y
// permisos. No depende de la base de datos, de HTTP ni de ninguna librería.
package acceso

import (
	"errors"
	"sort"
	"strings"
	"unicode/utf8"
)

var (
	ErrCredenciales = errors.New("email o contraseña incorrectos")
	ErrSinSesion    = errors.New("no hay sesión")
	ErrEmailEnUso   = errors.New("ya hay un usuario con ese email")
	ErrSinSucursal  = errors.New("el usuario no tiene acceso a esa sucursal")
	ErrClaveCorta   = errors.New("la contraseña tiene que tener al menos 8 caracteres")
	ErrClaveLarga   = errors.New("la contraseña es demasiado larga (máximo 72 caracteres)")
	ErrDatosAlta    = errors.New("faltan el nombre de la carpintería, el nombre o el email del admin")
	ErrNoEncontrado = errors.New("no encontrado")
)

// Sucursal de una carpintería.
type Sucursal struct {
	ID     int64  `json:"id"`
	Nombre string `json:"nombre"`
}

// Permisos es el conjunto de permisos de un usuario en una sucursal.
type Permisos map[string]bool

func (p Permisos) Tiene(codigo string) bool { return p[codigo] }

// Lista devuelve los permisos ordenados.
func (p Permisos) Lista() []string {
	out := make([]string, 0, len(p))
	for k := range p {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Usuario autenticado, con sus permisos en la sucursal elegida.
type Usuario struct {
	ID            int64
	Nombre        string
	Email         string
	CarpinteriaID int64
	Carpinteria   string
	SucursalID    int64
	Sucursales    []Sucursal
	Permisos      Permisos
}

// Asignacion es un permiso que el usuario tiene por alguno de sus roles.
// SucursalID nil significa que el rol vale para todas las sucursales.
type Asignacion struct {
	SucursalID *int64
	Permiso    string
}

// ResolverAcceso calcula a qué sucursales puede entrar el usuario, en cuál
// queda (la preferida si puede, si no la primera) y qué permisos tiene ahí.
// sucursalesActivas tiene que venir ordenada como se quiere mostrar.
func ResolverAcceso(sucursalesActivas []Sucursal, asignaciones []Asignacion, preferida *int64) (
	sucursales []Sucursal, elegida int64, permisos Permisos, err error) {

	global := false
	porSucursal := map[int64]bool{}
	for _, a := range asignaciones {
		if a.SucursalID == nil {
			global = true
		} else {
			porSucursal[*a.SucursalID] = true
		}
	}
	for _, s := range sucursalesActivas {
		if global || porSucursal[s.ID] {
			sucursales = append(sucursales, s)
		}
	}
	if len(sucursales) == 0 {
		return nil, 0, nil, ErrSinSesion
	}
	elegida = sucursales[0].ID
	if preferida != nil && Contiene(sucursales, *preferida) {
		elegida = *preferida
	}
	permisos = Permisos{}
	for _, a := range asignaciones {
		if a.SucursalID == nil || *a.SucursalID == elegida {
			permisos[a.Permiso] = true
		}
	}
	return sucursales, elegida, permisos, nil
}

func Contiene(ss []Sucursal, id int64) bool {
	for _, s := range ss {
		if s.ID == id {
			return true
		}
	}
	return false
}

// NormalizarEmail deja el email en minúsculas y sin espacios.
func NormalizarEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

// ValidarClave controla el largo de una contraseña nueva.
func ValidarClave(clave string) error {
	if utf8.RuneCountInString(clave) < 8 {
		return ErrClaveCorta
	}
	if len(clave) > 72 { // límite de bcrypt
		return ErrClaveLarga
	}
	return nil
}
