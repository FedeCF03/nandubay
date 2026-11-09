package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/FedeCF03/nandubay/backend/internal/auth"
	"github.com/FedeCF03/nandubay/backend/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Estos tests necesitan un Postgres vacío al que conectarse con un usuario
// SIN superusuario (si no, Row Level Security no se aplica):
//
//	NANDUBAY_TEST_DB=postgres://nandubay_test:clave@localhost:5432/nandubay_test go test ./...
//
// Borran y recrean el esquema public de esa base.
func baseDePrueba(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("NANDUBAY_TEST_DB")
	if url == "" {
		t.Skip("NANDUBAY_TEST_DB no está definida")
	}
	ctx := context.Background()
	pool, err := db.Conectar(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var superusuario bool
	if err := pool.QueryRow(ctx, "SELECT rolsuper FROM pg_roles WHERE rolname = current_user").Scan(&superusuario); err != nil {
		t.Fatal(err)
	}
	if superusuario {
		t.Fatal("NANDUBAY_TEST_DB tiene que usar un usuario sin superusuario")
	}
	if _, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrar(pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

type cliente struct {
	t   *testing.T
	srv *httptest.Server
	c   *http.Client
}

func nuevoCliente(t *testing.T, srv *httptest.Server) *cliente {
	jar, _ := cookiejar.New(nil)
	return &cliente{t, srv, &http.Client{Jar: jar}}
}

func (c *cliente) pedir(metodo, ruta string, cuerpo any) (int, map[string]any) {
	c.t.Helper()
	var b bytes.Buffer
	if cuerpo != nil {
		_ = json.NewEncoder(&b).Encode(cuerpo)
	}
	req, _ := http.NewRequest(metodo, c.srv.URL+ruta, &b)
	if metodo != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.c.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func preparar(t *testing.T) (*pgxpool.Pool, *auth.Servicio, *httptest.Server) {
	pool := baseDePrueba(t)
	svc := &auth.Servicio{DB: pool}
	s := &Servidor{DB: pool, Auth: svc, Limitador: auth.NuevoLimitador(5, time.Minute)}
	srv := httptest.NewServer(s.Router())
	t.Cleanup(srv.Close)
	return pool, svc, srv
}

func crear(t *testing.T, svc *auth.Servicio, nombre, email string) int64 {
	t.Helper()
	id, err := svc.CrearCarpinteria(context.Background(), auth.NuevaCarpinteria{
		Nombre: nombre, AdminNombre: "Admin " + nombre, AdminEmail: email, AdminClave: "clave-segura",
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// agregarUsuario crea un usuario con un rol base, en todas las sucursales.
func agregarUsuario(t *testing.T, pool *pgxpool.Pool, carpinteriaID int64, email, rol string) {
	t.Helper()
	hash, _ := auth.HashClave("clave-segura")
	err := db.EnCarpinteria(context.Background(), pool, carpinteriaID, func(tx pgx.Tx) error {
		var uid int64
		if err := tx.QueryRow(context.Background(),
			`INSERT INTO usuarios (carpinteria_id, email, nombre, password_hash) VALUES ($1,$2,$2,$3) RETURNING id`,
			carpinteriaID, email, hash).Scan(&uid); err != nil {
			return err
		}
		_, err := tx.Exec(context.Background(), `
			INSERT INTO usuario_roles (carpinteria_id, usuario_id, rol_id)
			SELECT $1, $2, id FROM roles WHERE nombre = $3`, carpinteriaID, uid, rol)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

var optimizarMinimo = map[string]any{
	"parametros": map[string]any{"placa_largo": 2600, "placa_ancho": 1830, "kerf": 3},
	"piezas":     []any{map[string]any{"id": "1", "etiqueta": "A", "largo": 500, "ancho": 300, "cantidad": 1}},
}

func TestLoginYPermisos(t *testing.T) {
	pool, svc, srv := preparar(t)
	id := crear(t, svc, "Carpintería López", "Juan@Ejemplo.com")
	agregarUsuario(t, pool, id, "deposito@ejemplo.com", "Depósito")

	c := nuevoCliente(t, srv)
	if st, _ := c.pedir("GET", "/api/v1/auth/yo", nil); st != http.StatusUnauthorized {
		t.Fatalf("sin sesión esperaba 401, obtuve %d", st)
	}
	if st, _ := c.pedir("POST", "/api/v1/auth/login", map[string]string{"email": "juan@ejemplo.com", "clave": "mal"}); st != http.StatusUnauthorized {
		t.Fatalf("con clave mala esperaba 401, obtuve %d", st)
	}
	// El email no distingue mayúsculas.
	if st, _ := c.pedir("POST", "/api/v1/auth/login", map[string]string{"email": " JUAN@ejemplo.com", "clave": "clave-segura"}); st != http.StatusNoContent {
		t.Fatalf("login esperaba 204, obtuve %d", st)
	}
	st, yo := c.pedir("GET", "/api/v1/auth/yo", nil)
	if st != http.StatusOK || yo["carpinteria"] != "Carpintería López" || len(yo["sucursales"].([]any)) != 1 {
		t.Fatalf("yo inesperado: %d %v", st, yo)
	}
	if st, _ := c.pedir("POST", "/api/v1/cortes/optimizar", optimizarMinimo); st != http.StatusOK {
		t.Fatalf("el admin debería poder optimizar, obtuve %d", st)
	}
	if st, _ := c.pedir("POST", "/api/v1/auth/sucursal", map[string]int{"sucursal_id": 999}); st != http.StatusForbidden {
		t.Fatalf("sucursal ajena esperaba 403, obtuve %d", st)
	}
	if st, _ := c.pedir("POST", "/api/v1/auth/logout", nil); st != http.StatusNoContent {
		t.Fatalf("logout esperaba 204, obtuve %d", st)
	}
	if st, _ := c.pedir("GET", "/api/v1/auth/yo", nil); st != http.StatusUnauthorized {
		t.Fatalf("después del logout esperaba 401, obtuve %d", st)
	}

	// El rol Depósito no tiene cortes.generar.
	d := nuevoCliente(t, srv)
	d.pedir("POST", "/api/v1/auth/login", map[string]string{"email": "deposito@ejemplo.com", "clave": "clave-segura"})
	if st, _ := d.pedir("POST", "/api/v1/cortes/optimizar", optimizarMinimo); st != http.StatusForbidden {
		t.Fatalf("depósito no debería poder optimizar, obtuve %d", st)
	}
}

func TestPermisosPorSucursal(t *testing.T) {
	pool, svc, srv := preparar(t)
	id := crear(t, svc, "Carpintería López", "juan@ejemplo.com")
	ctx := context.Background()
	hash, _ := auth.HashClave("clave-segura")
	var norte, centro int64
	// Vendedor solo en la sucursal Norte.
	err := db.EnCarpinteria(ctx, pool, id, func(tx pgx.Tx) error {
		tx.QueryRow(ctx, `SELECT id FROM sucursales`).Scan(&centro)
		tx.QueryRow(ctx, `INSERT INTO sucursales (carpinteria_id, nombre) VALUES ($1, 'Norte') RETURNING id`, id).Scan(&norte)
		var uid int64
		tx.QueryRow(ctx, `INSERT INTO usuarios (carpinteria_id, email, nombre, password_hash) VALUES ($1,'v@e.com','V',$2) RETURNING id`, id, hash).Scan(&uid)
		_, err := tx.Exec(ctx, `INSERT INTO usuario_roles (carpinteria_id, usuario_id, rol_id, sucursal_id)
			SELECT $1, $2, id, $3 FROM roles WHERE nombre = 'Vendedor'`, id, uid, norte)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	c := nuevoCliente(t, srv)
	c.pedir("POST", "/api/v1/auth/login", map[string]string{"email": "v@e.com", "clave": "clave-segura"})
	_, yo := c.pedir("GET", "/api/v1/auth/yo", nil)
	if s := yo["sucursales"].([]any); len(s) != 1 || int64(yo["sucursal_id"].(float64)) != norte {
		t.Fatalf("el vendedor solo debería ver la sucursal Norte: %v", yo)
	}
	if st, _ := c.pedir("POST", "/api/v1/auth/sucursal", map[string]int64{"sucursal_id": centro}); st != http.StatusForbidden {
		t.Fatalf("no debería poder pasarse a Centro, obtuve %d", st)
	}
}

func TestAislamientoEntreCarpinterias(t *testing.T) {
	pool, svc, _ := preparar(t)
	a := crear(t, svc, "A", "a@ejemplo.com")
	b := crear(t, svc, "B", "b@ejemplo.com")
	ctx := context.Background()
	if err := db.EnCarpinteria(ctx, pool, b, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO clientes (carpinteria_id, nombre) VALUES ($1, 'Cliente de B')`, b)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = db.EnCarpinteria(ctx, pool, a, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM clientes`).Scan(&n)
	})
	if n != 0 {
		t.Fatalf("A no debería ver clientes de B, ve %d", n)
	}
	err := db.EnCarpinteria(ctx, pool, a, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO clientes (carpinteria_id, nombre) VALUES ($1, 'Intruso')`, b)
		return err
	})
	if err == nil {
		t.Fatal("A no debería poder escribir datos de B")
	}
	// El email es único en todo el sistema.
	_, err = svc.CrearCarpinteria(ctx, auth.NuevaCarpinteria{Nombre: "C", AdminNombre: "x", AdminEmail: "A@ejemplo.com", AdminClave: "clave-segura"})
	if err != auth.ErrEmailEnUso {
		t.Fatalf("esperaba ErrEmailEnUso, obtuve %v", err)
	}
}

func TestLimiteDeIntentos(t *testing.T) {
	_, svc, srv := preparar(t)
	crear(t, svc, "A", "a@ejemplo.com")
	c := nuevoCliente(t, srv)
	for i := 0; i < 5; i++ {
		c.pedir("POST", "/api/v1/auth/login", map[string]string{"email": "a@ejemplo.com", "clave": "mal"})
	}
	if st, _ := c.pedir("POST", "/api/v1/auth/login", map[string]string{"email": "a@ejemplo.com", "clave": "clave-segura"}); st != http.StatusTooManyRequests {
		t.Fatalf("esperaba 429, obtuve %d", st)
	}
}

func TestRechazaFormularios(t *testing.T) {
	_, _, srv := preparar(t)
	res, err := http.Post(srv.URL+"/api/v1/auth/login", "application/x-www-form-urlencoded", bytes.NewBufferString("email=a"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("esperaba 415, obtuve %d", res.StatusCode)
	}
}
