package handler

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/FedeCF03/nandubay/backend/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func depositoNuevo(t *testing.T, pool *pgxpool.Pool, carpinteriaID int64, sucursal string) int64 {
	t.Helper()
	var id int64
	ctx := context.Background()
	err := store.EnCarpinteria(ctx, pool, carpinteriaID, func(tx pgx.Tx) error {
		var sid int64
		if err := tx.QueryRow(ctx, `INSERT INTO sucursales (carpinteria_id, nombre) VALUES ($1, $2) RETURNING id`, carpinteriaID, sucursal).Scan(&sid); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `INSERT INTO depositos (carpinteria_id, sucursal_id, nombre) VALUES ($1, $2, $3) RETURNING id`,
			carpinteriaID, sid, "Depósito "+sucursal).Scan(&id)
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func entrar(t *testing.T, c *cliente, email string) {
	t.Helper()
	if st, _ := c.pedir("POST", "/api/v1/auth/login", map[string]string{"email": email, "clave": "clave-segura"}); st != http.StatusNoContent {
		t.Fatalf("login de %s: %d", email, st)
	}
}

func listar(t *testing.T, c *cliente) []map[string]any {
	t.Helper()
	req, _ := http.NewRequest("GET", c.srv.URL+"/api/v1/productos", nil)
	res, err := c.c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out []map[string]any
	_ = jsonDecode(res, &out)
	return out
}

var placa = map[string]any{
	"tipo": "placa", "sku": "MEL-BL-18", "nombre": "Melamina Blanca 18 mm",
	"largo_mm": 2600, "ancho_mm": 1830, "espesor_mm": 18, "costo": 50000, "precio_venta": 70000, "stock_minimo": 5,
}

func TestStockCompleto(t *testing.T) {
	pool, svc, srv := preparar(t)
	id := crear(t, svc, "López", "juan@ejemplo.com")
	gonnet := depositoNuevo(t, pool, id, "Gonnet")
	var principal int64
	_ = store.EnCarpinteria(context.Background(), pool, id, func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `SELECT id FROM depositos WHERE nombre = 'Depósito principal'`).Scan(&principal)
	})

	c := nuevoCliente(t, srv)
	entrar(t, c, "juan@ejemplo.com")

	if st, _ := c.pedir("POST", "/api/v1/productos", map[string]any{"tipo": "placa", "nombre": "Sin medidas"}); st != http.StatusUnprocessableEntity {
		t.Fatalf("placa sin medidas esperaba 422, obtuve %d", st)
	}
	st, prod := c.pedir("POST", "/api/v1/productos", placa)
	if st != http.StatusCreated {
		t.Fatalf("crear producto: %d %v", st, prod)
	}
	if st, _ := c.pedir("POST", "/api/v1/productos", placa); st != http.StatusConflict {
		t.Fatalf("sku repetido esperaba 409, obtuve %d", st)
	}
	pid := int64(prod["id"].(float64))
	mov := func(ruta string, cuerpo map[string]any) (int, map[string]any) {
		cuerpo["producto_id"] = pid
		return c.pedir("POST", "/api/v1/stock/"+ruta, cuerpo)
	}

	if st, _ := mov("ingresos", map[string]any{"deposito_id": principal, "cantidad": 10, "costo_unitario": 52000}); st != http.StatusNoContent {
		t.Fatalf("ingreso: %d", st)
	}
	if st, _ := mov("ajustes", map[string]any{"deposito_id": principal, "cantidad": 8}); st != http.StatusUnprocessableEntity {
		t.Fatalf("ajuste sin motivo esperaba 422, obtuve %d", st)
	}
	if st, _ := mov("ajustes", map[string]any{"deposito_id": principal, "cantidad": 8, "nota": "2 rotas"}); st != http.StatusNoContent {
		t.Fatalf("ajuste: %d", st)
	}
	if st, _ := mov("traspasos", map[string]any{"deposito_id": principal, "destino_id": gonnet, "cantidad": 10}); st != http.StatusUnprocessableEntity {
		t.Fatalf("traspaso sin stock esperaba 422, obtuve %d", st)
	}
	if st, _ := mov("traspasos", map[string]any{"deposito_id": principal, "destino_id": gonnet, "cantidad": 5}); st != http.StatusNoContent {
		t.Fatalf("traspaso: %d", st)
	}

	ps := listar(t, c)
	if len(ps) != 1 {
		t.Fatalf("esperaba 1 producto, hay %d", len(ps))
	}
	p := ps[0]
	pd := p["por_deposito"].(map[string]any)
	if p["total"].(float64) != 8 || pd[fmt.Sprint(principal)].(float64) != 3 || pd[fmt.Sprint(gonnet)].(float64) != 5 {
		t.Fatalf("stock inesperado: %v", p)
	}
	if p["costo"].(float64) != 52000 || p["bajo"] != false {
		t.Fatalf("el ingreso debería actualizar el costo y no estar bajo: %v", p)
	}

	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/productos/%d/movimientos", srv.URL, pid), nil)
	res, _ := c.c.Do(req)
	var ms []map[string]any
	_ = jsonDecode(res, &ms)
	res.Body.Close()
	if len(ms) != 4 || ms[3]["tipo"] != "compra" || ms[2]["cantidad"].(float64) != -2 || ms[2]["usuario"] != "Admin López" {
		t.Fatalf("movimientos inesperados: %v", ms)
	}

	// Otra carpintería no ve el producto ni puede usar sus depósitos.
	otra := crear(t, svc, "Otra", "otra@ejemplo.com")
	_ = otra
	o := nuevoCliente(t, srv)
	entrar(t, o, "otra@ejemplo.com")
	if len(listar(t, o)) != 0 {
		t.Fatal("la otra carpintería no debería ver productos ajenos")
	}
	if st, _ := o.pedir("POST", "/api/v1/stock/ingresos", map[string]any{"producto_id": pid, "deposito_id": principal, "cantidad": 1}); st != http.StatusNotFound {
		t.Fatalf("producto ajeno esperaba 404, obtuve %d", st)
	}
	_, prodOtra := o.pedir("POST", "/api/v1/productos", placa)
	if st, _ := o.pedir("POST", "/api/v1/stock/ingresos", map[string]any{"producto_id": prodOtra["id"], "deposito_id": principal, "cantidad": 1}); st != http.StatusNotFound {
		t.Fatalf("depósito ajeno esperaba 404, obtuve %d", st)
	}
}

func TestPermisosDeStock(t *testing.T) {
	pool, svc, srv := preparar(t)
	id := crear(t, svc, "López", "juan@ejemplo.com")
	agregarUsuario(t, pool, id, "vendedor@ejemplo.com", "Vendedor")
	agregarUsuario(t, pool, id, "deposito@ejemplo.com", "Depósito")

	v := nuevoCliente(t, srv)
	entrar(t, v, "vendedor@ejemplo.com")
	if st, _ := v.pedir("POST", "/api/v1/productos", placa); st != http.StatusForbidden {
		t.Fatalf("el vendedor no crea productos, obtuve %d", st)
	}
	if st, _ := v.pedir("POST", "/api/v1/stock/ingresos", map[string]any{"producto_id": 1, "deposito_id": 1, "cantidad": 1}); st != http.StatusForbidden {
		t.Fatalf("el vendedor no ingresa stock, obtuve %d", st)
	}
	d := nuevoCliente(t, srv)
	entrar(t, d, "deposito@ejemplo.com")
	if st, _ := d.pedir("POST", "/api/v1/productos", placa); st != http.StatusCreated {
		t.Fatalf("depósito sí crea productos, obtuve %d", st)
	}
	if len(listar(t, v)) != 1 {
		t.Fatal("el vendedor sí ve productos")
	}
}

// Veinte salidas de 1 al mismo tiempo sobre un stock de 10: tienen que
// pasar exactamente 10 y el stock quedar en cero, nunca negativo.
func TestSalidasConcurrentes(t *testing.T) {
	pool, svc, srv := preparar(t)
	id := crear(t, svc, "López", "juan@ejemplo.com")
	destino := depositoNuevo(t, pool, id, "Gonnet")
	c := nuevoCliente(t, srv)
	entrar(t, c, "juan@ejemplo.com")
	_, prod := c.pedir("POST", "/api/v1/productos", placa)
	pid := prod["id"]
	var origen int64
	_ = store.EnCarpinteria(context.Background(), pool, id, func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `SELECT id FROM depositos WHERE nombre = 'Depósito principal'`).Scan(&origen)
	})
	c.pedir("POST", "/api/v1/stock/ingresos", map[string]any{"producto_id": pid, "deposito_id": origen, "cantidad": 10})

	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			st, _ := c.pedir("POST", "/api/v1/stock/traspasos", map[string]any{"producto_id": pid, "deposito_id": origen, "destino_id": destino, "cantidad": 1})
			if st == http.StatusNoContent {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	p := listar(t, c)[0]["por_deposito"].(map[string]any)
	if ok != 10 || p[fmt.Sprint(origen)].(float64) != 0 || p[fmt.Sprint(destino)].(float64) != 10 {
		t.Fatalf("pasaron %d traspasos; stock %v", ok, p)
	}
}
