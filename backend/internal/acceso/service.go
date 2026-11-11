package acceso

import (
	"context"
	"errors"
	"strings"
	"time"
)

const DuracionSesion = 30 * 24 * time.Hour

// Servicio implementa login, sesiones, permisos y alta de carpinterías.
type Servicio struct {
	Store  Store
	Hasher Hasher
	Tokens Tokens
	Ahora  Reloj
}

func (s *Servicio) ahora() time.Time {
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
func (s *Servicio) CrearCarpinteria(ctx context.Context, n NuevaCarpinteria) (int64, error) {
	alta := AltaCarpinteria{
		Nombre:      strings.TrimSpace(n.Nombre),
		Sucursal:    strings.TrimSpace(n.Sucursal),
		Deposito:    "Depósito principal",
		Roles:       RolesBase,
		RolAdmin:    NombreRolAdmin,
		AdminNombre: strings.TrimSpace(n.AdminNombre),
		AdminEmail:  NormalizarEmail(n.AdminEmail),
	}
	if alta.Nombre == "" || alta.AdminNombre == "" || !strings.Contains(alta.AdminEmail, "@") {
		return 0, ErrDatosAlta
	}
	if alta.Sucursal == "" {
		alta.Sucursal = "Casa central"
	}
	if err := ValidarClave(n.AdminClave); err != nil {
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
func (s *Servicio) Login(ctx context.Context, email, clave, ip, agente string) (string, error) {
	cuenta, err := s.Store.BuscarCuenta(ctx, NormalizarEmail(email))
	if errors.Is(err, ErrNoEncontrado) {
		s.Hasher.Comparar("", clave) // misma demora que con un email existente
		return "", ErrCredenciales
	}
	if err != nil {
		return "", err
	}
	if !s.Hasher.Comparar(cuenta.HashClave, clave) || !cuenta.Activo {
		return "", ErrCredenciales
	}

	token, hash, err := s.Tokens.Nuevo()
	if err != nil {
		return "", err
	}
	if len(agente) > 300 {
		agente = agente[:300]
	}
	ahora := s.ahora()
	err = s.Store.GuardarSesion(ctx, Sesion{
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

func (s *Servicio) Logout(ctx context.Context, token string) error {
	return s.Store.BorrarSesion(ctx, s.Tokens.Hash(token))
}

// Autenticar devuelve el usuario dueño del token con sus permisos en la
// sucursal elegida en la sesión.
func (s *Servicio) Autenticar(ctx context.Context, token string) (*Usuario, error) {
	if token == "" {
		return nil, ErrSinSesion
	}
	ses, err := s.Store.UsarSesion(ctx, s.Tokens.Hash(token), s.ahora())
	if errors.Is(err, ErrNoEncontrado) {
		return nil, ErrSinSesion
	}
	if err != nil {
		return nil, err
	}
	d, err := s.Store.CargarAcceso(ctx, ses.CarpinteriaID, ses.UsuarioID)
	if errors.Is(err, ErrNoEncontrado) {
		return nil, ErrSinSesion
	}
	if err != nil {
		return nil, err
	}
	if !d.Activo {
		return nil, ErrSinSesion
	}
	sucursales, elegida, permisos, err := ResolverAcceso(d.SucursalesActivas, d.Asignaciones, ses.SucursalID)
	if err != nil {
		return nil, err
	}
	return &Usuario{
		ID: ses.UsuarioID, Nombre: d.Nombre, Email: d.Email,
		CarpinteriaID: ses.CarpinteriaID, Carpinteria: d.Carpinteria,
		SucursalID: elegida, Sucursales: sucursales, Permisos: permisos,
	}, nil
}

// ElegirSucursal cambia la sucursal de la sesión del token.
func (s *Servicio) ElegirSucursal(ctx context.Context, u *Usuario, token string, sucursalID int64) error {
	if !Contiene(u.Sucursales, sucursalID) {
		return ErrSinSucursal
	}
	return s.Store.CambiarSucursalSesion(ctx, s.Tokens.Hash(token), sucursalID)
}
