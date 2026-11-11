package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/FedeCF03/nandubay/backend/internal/model/acceso"
)

const DuracionSesion = 30 * 24 * time.Hour

// ServicioAcceso implementa login, sesiones, permisos y alta de carpinterías.
type ServicioAcceso struct {
	Store  StoreAcceso
	Hasher Hasher
	Tokens Tokens
	Ahora  Reloj
}

func (s *ServicioAcceso) ahora() time.Time {
	if s.Ahora != nil {
		return s.Ahora()
	}
	return time.Now()
}

// NuevaCarpinteria son los datos del formulario o comando de alta.
type NuevaCarpinteria struct {
	Nombre      string
	Sucursal    string
	AdminNombre string
	AdminEmail  string
	AdminClave  string
}

// CrearCarpinteria da de alta una carpintería con una sucursal, un depósito,
// los roles base y su usuario administrador.
func (s *ServicioAcceso) CrearCarpinteria(ctx context.Context, n NuevaCarpinteria) (int64, error) {
	alta := acceso.AltaCarpinteria{
		Nombre:      strings.TrimSpace(n.Nombre),
		Sucursal:    strings.TrimSpace(n.Sucursal),
		Deposito:    "Depósito principal",
		Roles:       acceso.RolesBase,
		RolAdmin:    acceso.NombreRolAdmin,
		AdminNombre: strings.TrimSpace(n.AdminNombre),
		AdminEmail:  acceso.NormalizarEmail(n.AdminEmail),
	}
	if alta.Nombre == "" || alta.AdminNombre == "" || !strings.Contains(alta.AdminEmail, "@") {
		return 0, acceso.ErrDatosAlta
	}
	if alta.Sucursal == "" {
		alta.Sucursal = "Casa central"
	}
	if err := acceso.ValidarClave(n.AdminClave); err != nil {
		return 0, err
	}
	hash, err := s.Hasher.Hash(n.AdminClave)
	if err != nil {
		return 0, err
	}
	alta.AdminHashClave = hash
	return s.Store.CrearCarpinteria(ctx, alta)
}

// Login valida email y contraseña y abre una sesión. Devuelve el token.
func (s *ServicioAcceso) Login(ctx context.Context, email, clave, ip, agente string) (string, error) {
	cuenta, err := s.Store.BuscarCuenta(ctx, acceso.NormalizarEmail(email))
	if errors.Is(err, acceso.ErrNoEncontrado) {
		s.Hasher.Comparar("", clave) // misma demora que con un email existente
		return "", acceso.ErrCredenciales
	}
	if err != nil {
		return "", err
	}
	if !s.Hasher.Comparar(cuenta.HashClave, clave) || !cuenta.Activo {
		return "", acceso.ErrCredenciales
	}

	token, hash, err := s.Tokens.Nuevo()
	if err != nil {
		return "", err
	}
	if len(agente) > 300 {
		agente = agente[:300]
	}
	ahora := s.ahora()
	err = s.Store.GuardarSesion(ctx, acceso.Sesion{
		TokenHash: hash, UsuarioID: cuenta.UsuarioID, CarpinteriaID: cuenta.CarpinteriaID,
		ExpiraEn: ahora.Add(DuracionSesion), IP: ip, Agente: agente,
	})
	if err != nil {
		return "", err
	}
	_ = s.Store.RegistrarIngreso(ctx, cuenta.CarpinteriaID, cuenta.UsuarioID)
	_ = s.Store.BorrarSesionesVencidas(ctx, ahora)
	return token, nil
}

func (s *ServicioAcceso) Logout(ctx context.Context, token string) error {
	return s.Store.BorrarSesion(ctx, s.Tokens.Hash(token))
}

// Autenticar devuelve el usuario dueño del token con sus permisos en la
// sucursal elegida en la sesión.
func (s *ServicioAcceso) Autenticar(ctx context.Context, token string) (*acceso.Usuario, error) {
	if token == "" {
		return nil, acceso.ErrSinSesion
	}
	ses, err := s.Store.UsarSesion(ctx, s.Tokens.Hash(token), s.ahora())
	if errors.Is(err, acceso.ErrNoEncontrado) {
		return nil, acceso.ErrSinSesion
	}
	if err != nil {
		return nil, err
	}
	d, err := s.Store.CargarAcceso(ctx, ses.CarpinteriaID, ses.UsuarioID)
	if errors.Is(err, acceso.ErrNoEncontrado) {
		return nil, acceso.ErrSinSesion
	}
	if err != nil {
		return nil, err
	}
	if !d.Activo {
		return nil, acceso.ErrSinSesion
	}
	sucursales, elegida, permisos, err := acceso.ResolverAcceso(d.SucursalesActivas, d.Asignaciones, ses.SucursalID)
	if err != nil {
		return nil, err
	}
	return &acceso.Usuario{
		ID: ses.UsuarioID, Nombre: d.Nombre, Email: d.Email,
		CarpinteriaID: ses.CarpinteriaID, Carpinteria: d.Carpinteria,
		SucursalID: elegida, Sucursales: sucursales, Permisos: permisos,
	}, nil
}

// ElegirSucursal cambia la sucursal de la sesión del token.
func (s *ServicioAcceso) ElegirSucursal(ctx context.Context, u *acceso.Usuario, token string, sucursalID int64) error {
	if !acceso.Contiene(u.Sucursales, sucursalID) {
		return acceso.ErrSinSucursal
	}
	return s.Store.CambiarSucursalSesion(ctx, s.Tokens.Hash(token), sucursalID)
}
