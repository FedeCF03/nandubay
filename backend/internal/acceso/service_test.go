package acceso

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Tests de los casos de uso con dobles en memoria: no necesitan Postgres.

type repoFalso struct {
	cuentas  map[string]CuentaLogin
	accesos  map[int64]DatosAcceso
	sesiones map[string]Sesion
	altas    []AltaCarpinteria
}

func nuevoRepo() *repoFalso {
	return &repoFalso{cuentas: map[string]CuentaLogin{}, accesos: map[int64]DatosAcceso{}, sesiones: map[string]Sesion{}}
}

func (r *repoFalso) BuscarCuenta(_ context.Context, email string) (CuentaLogin, error) {
	c, ok := r.cuentas[email]
	if !ok {
		return c, ErrNoEncontrado
	}
	return c, nil
}
func (r *repoFalso) RegistrarIngreso(context.Context, int64, int64) error { return nil }
func (r *repoFalso) CargarAcceso(_ context.Context, _, usuarioID int64) (DatosAcceso, error) {
	d, ok := r.accesos[usuarioID]
	if !ok {
		return d, ErrNoEncontrado
	}
	return d, nil
}
func (r *repoFalso) CrearCarpinteria(_ context.Context, a AltaCarpinteria) (int64, error) {
	r.altas = append(r.altas, a)
	return int64(len(r.altas)), nil
}
func (r *repoFalso) GuardarSesion(_ context.Context, s Sesion) error {
	r.sesiones[s.TokenHash] = s
	return nil
}
func (r *repoFalso) UsarSesion(_ context.Context, h string, ahora time.Time) (Sesion, error) {
	s, ok := r.sesiones[h]
	if !ok || !s.ExpiraEn.After(ahora) {
		return s, ErrNoEncontrado
	}
	return s, nil
}
func (r *repoFalso) CambiarSucursalSesion(_ context.Context, h string, id int64) error {
	s := r.sesiones[h]
	s.SucursalID = &id
	r.sesiones[h] = s
	return nil
}
func (r *repoFalso) BorrarSesion(_ context.Context, h string) error {
	delete(r.sesiones, h)
	return nil
}
func (r *repoFalso) BorrarSesionesVencidas(context.Context, time.Time) error { return nil }

type hasherFalso struct{}

func (hasherFalso) Hash(c string) (string, error) { return "h:" + c, nil }
func (hasherFalso) Comparar(h, c string) bool     { return h != "" && h == "h:"+c }

type tokensFalsos struct{ n int }

func (t *tokensFalsos) Nuevo() (string, string, error) {
	t.n++
	tok := string(rune('a' + t.n))
	return tok, t.Hash(tok), nil
}
func (*tokensFalsos) Hash(tok string) string { return "#" + tok }

func servicio() (*Servicio, *repoFalso, *time.Time) {
	repo := nuevoRepo()
	ahora := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	s := &Servicio{Store: repo, Hasher: hasherFalso{}, Tokens: &tokensFalsos{}, Ahora: func() time.Time { return ahora }}
	repo.cuentas["juan@ejemplo.com"] = CuentaLogin{UsuarioID: 7, CarpinteriaID: 1, HashClave: "h:clave-segura", Activo: true}
	norte := int64(2)
	repo.accesos[7] = DatosAcceso{
		Nombre: "Juan", Email: "juan@ejemplo.com", Activo: true, Carpinteria: "López",
		SucursalesActivas: []Sucursal{{ID: 1, Nombre: "Centro"}, {ID: 2, Nombre: "Norte"}},
		Asignaciones:      []Asignacion{{SucursalID: nil, Permiso: "cortes.ver"}, {SucursalID: &norte, Permiso: "cortes.generar"}},
	}
	return s, repo, &ahora
}

func TestLoginYAutenticar(t *testing.T) {
	s, _, _ := servicio()
	ctx := context.Background()
	if _, err := s.Login(ctx, "juan@ejemplo.com", "mala", "", ""); !errors.Is(err, ErrCredenciales) {
		t.Fatalf("clave mala: %v", err)
	}
	if _, err := s.Login(ctx, "nadie@ejemplo.com", "clave-segura", "", ""); !errors.Is(err, ErrCredenciales) {
		t.Fatalf("email inexistente: %v", err)
	}
	tok, err := s.Login(ctx, "  JUAN@ejemplo.com ", "clave-segura", "", "")
	if err != nil {
		t.Fatal(err)
	}
	u, err := s.Autenticar(ctx, tok)
	if err != nil || u.ID != 7 || u.SucursalID != 1 || u.Permisos.Tiene("cortes.generar") {
		t.Fatalf("en Centro no debería generar cortes: %+v %v", u, err)
	}
	if err := s.ElegirSucursal(ctx, u, tok, 2); err != nil {
		t.Fatal(err)
	}
	u, _ = s.Autenticar(ctx, tok)
	if u.SucursalID != 2 || !u.Permisos.Tiene("cortes.generar") {
		t.Fatalf("en Norte sí: %+v", u)
	}
	if err := s.ElegirSucursal(ctx, u, tok, 99); !errors.Is(err, ErrSinSucursal) {
		t.Fatalf("sucursal ajena: %v", err)
	}
	_ = s.Logout(ctx, tok)
	if _, err := s.Autenticar(ctx, tok); !errors.Is(err, ErrSinSesion) {
		t.Fatalf("después de logout: %v", err)
	}
}

func TestSesionVencida(t *testing.T) {
	s, _, ahora := servicio()
	ctx := context.Background()
	tok, _ := s.Login(ctx, "juan@ejemplo.com", "clave-segura", "", "")
	*ahora = ahora.Add(DuracionSesion + time.Minute)
	if _, err := s.Autenticar(ctx, tok); !errors.Is(err, ErrSinSesion) {
		t.Fatalf("la sesión debería estar vencida: %v", err)
	}
}

func TestUsuarioInactivo(t *testing.T) {
	s, repo, _ := servicio()
	ctx := context.Background()
	c := repo.cuentas["juan@ejemplo.com"]
	c.Activo = false
	repo.cuentas["juan@ejemplo.com"] = c
	if _, err := s.Login(ctx, "juan@ejemplo.com", "clave-segura", "", ""); !errors.Is(err, ErrCredenciales) {
		t.Fatalf("un usuario inactivo no entra: %v", err)
	}
}

func TestCrearCarpinteria(t *testing.T) {
	s, repo, _ := servicio()
	ctx := context.Background()
	if _, err := s.CrearCarpinteria(ctx, NuevaCarpinteria{Nombre: "X", AdminNombre: "A", AdminEmail: "sin-arroba", AdminClave: "clave-segura"}); !errors.Is(err, ErrDatosAlta) {
		t.Fatalf("email inválido: %v", err)
	}
	if _, err := s.CrearCarpinteria(ctx, NuevaCarpinteria{Nombre: "X", AdminNombre: "A", AdminEmail: "a@b.com", AdminClave: "corta"}); !errors.Is(err, ErrClaveCorta) {
		t.Fatalf("clave corta: %v", err)
	}
	if _, err := s.CrearCarpinteria(ctx, NuevaCarpinteria{Nombre: " López ", AdminNombre: "Juan", AdminEmail: " A@B.com", AdminClave: "clave-segura"}); err != nil {
		t.Fatal(err)
	}
	a := repo.altas[0]
	if a.Nombre != "López" || a.AdminEmail != "a@b.com" || a.Sucursal != "Casa central" ||
		a.AdminHashClave != "h:clave-segura" || len(a.Roles) != 4 || a.RolAdmin != "Admin" {
		t.Fatalf("alta inesperada: %+v", a)
	}
}
