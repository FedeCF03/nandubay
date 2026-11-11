package handler

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/FedeCF03/nandubay/backend/internal/service"
	"github.com/FedeCF03/nandubay/backend/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func idDeJSON(m map[string]any) int64 { return int64(m["id"].(float64)) }

func sucursalDeDeposito(t *testing.T, pool *pgxpool.Pool, carpinteriaID, depositoID int64) int64 {
	t.Helper()
	var id int64
	err := store.EnCarpinteria(context.Background(), pool, carpinteriaID, func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `SELECT sucursal_id FROM depositos WHERE id = $1`, depositoID).Scan(&id)
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func stockEn(t *testing.T, c *cliente, productoID, depositoID int64) float64 {
	t.Helper()
	for _, p := range listar(t, c) {
		if int64(p["id"].(float64)) == productoID {
			v, _ := p["por_deposito"].(map[string]any)[fmt.Sprint(depositoID)].(float64)
			return v
		}
	}
	t.Fatalf("no está el producto %d", productoID)
	return 0
}

// productoEnStock devuelve el producto como lo muestra la pantalla de stock.
func productoEnStock(t *testing.T, c *cliente, productoID int64) map[string]any {
	t.Helper()
	for _, p := range listar(t, c) {
		if int64(p["id"].(float64)) == productoID {
			return p
		}
	}
	t.Fatalf("no está el producto %d", productoID)
	return nil
}

func (c *cliente) pedirLista(metodo, ruta string) (int, []map[string]any) {
	c.t.Helper()
	req, _ := http.NewRequest(metodo, c.srv.URL+ruta, nil)
	res, err := c.c.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	var out []map[string]any
	_ = jsonDecode(res, &out)
	return res.StatusCode, out
}

// prepararVenta arma una carpintería con un depósito en La Plata con 5 placas.
func prepararVenta(t *testing.T) (pool *pgxpool.Pool, svc *service.ServicioAcceso, c *cliente, carpID, placaID, principal, laPlata int64) {
	t.Helper()
	pool, svc, srv := preparar(t)
	carpID = crear(t, svc, "López", "juan@ejemplo.com")
	laPlata = depositoNuevo(t, pool, carpID, "La Plata")
	_ = store.EnCarpinteria(context.Background(), pool, carpID, func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `SELECT id FROM depositos WHERE nombre = 'Depósito principal'`).Scan(&principal)
	})
	c = nuevoCliente(t, srv)
	entrar(t, c, "juan@ejemplo.com")
	_, prod := c.pedir("POST", "/api/v1/productos", placa)
	placaID = idDeJSON(prod)
	if st, r := c.pedir("POST", "/api/v1/stock/ingresos", map[string]any{"producto_id": placaID, "deposito_id": laPlata, "cantidad": 5}); st != http.StatusNoContent && st != http.StatusOK {
		t.Fatalf("ingreso: %d %v", st, r)
	}
	return
}

func TestVentaCompraEnUnaSucursalRetiraEnOtra(t *testing.T) {
	pool, _, c, carpID, placaID, principal, laPlata := prepararVenta(t)

	if st, _ := c.pedir("POST", "/api/v1/clientes", map[string]any{"nombre": " "}); st != http.StatusUnprocessableEntity {
		t.Fatalf("cliente sin nombre: %d", st)
	}
	_, cli := c.pedir("POST", "/api/v1/clientes", map[string]any{"nombre": "María Gómez", "documento": "30111222", "telefono": "221 555 1234"})
	cliID := idDeJSON(cli)

	if st, _ := c.pedir("POST", "/api/v1/ventas", map[string]any{"cliente_id": cliID, "lineas": []any{}}); st != http.StatusUnprocessableEntity {
		t.Fatalf("venta vacía: %d", st)
	}
	st, v := c.pedir("POST", "/api/v1/ventas", map[string]any{
		"cliente_id": cliID, "notas": "Llamar antes", "fecha_entrega": "2026-12-01",
		"lineas": []any{
			map[string]any{"tipo": "producto", "producto_id": placaID, "cantidad": 3},
			map[string]any{"tipo": "producto", "producto_id": placaID, "descripcion": "Placa de exhibición", "cantidad": 1, "precio_unitario": 90000},
		},
	})
	if st != http.StatusCreated {
		t.Fatalf("crear presupuesto: %d %v", st, v)
	}
	if v["tipo"] != "mostrador" {
		t.Errorf("tipo = %v", v["tipo"])
	}
	if st, _ := c.pedir("POST", "/api/v1/ventas", map[string]any{"cliente_id": cliID, "lineas": []any{
		map[string]any{"tipo": "mueble", "descripcion": "Bajo mesada", "cantidad": 1, "precio_unitario": 90000},
	}}); st != http.StatusUnprocessableEntity {
		t.Fatalf("un mueble en el mostrador: %d", st)
	}
	vid := idDeJSON(v)
	ruta := fmt.Sprintf("/api/v1/ventas/%d", vid)
	if v["numero"].(float64) != 1 || v["estado"] != "presupuesto" || v["total"].(float64) != 3*70000+90000 || v["pagada"] != false {
		t.Fatalf("presupuesto = %v", v)
	}
	if v["fecha_entrega"] != "2026-12-01" {
		t.Errorf("fecha de entrega = %v", v["fecha_entrega"])
	}

	// Editar mientras es presupuesto.
	st, v = c.pedir("PUT", ruta, map[string]any{
		"cliente_id": cliID,
		"lineas": []any{
			map[string]any{"tipo": "producto", "producto_id": placaID, "cantidad": 3, "precio_unitario": 65000},
			map[string]any{"tipo": "producto", "producto_id": placaID, "descripcion": "Placa de exhibición", "cantidad": 1, "precio_unitario": 90000},
		},
	})
	if st != http.StatusOK || v["total"].(float64) != 3*65000+90000 {
		t.Fatalf("editar: %d %v", st, v)
	}
	total := v["total"].(float64)

	// En el depósito principal no hay stock: no se confirma y no se toca nada.
	if st, r := c.pedir("POST", ruta+"/confirmar", map[string]any{"deposito_id": principal}); st != http.StatusUnprocessableEntity {
		t.Fatalf("confirmar sin stock: %d %v", st, r)
	}
	// Dónde está guardada la placa en La Plata: sale en la venta.
	if st, r := c.pedir("PUT", fmt.Sprintf("/api/v1/productos/%d/ubicaciones", placaID), map[string]any{
		"ubicaciones": []any{map[string]any{"deposito_id": laPlata, "ubicacion": "  Estante   B3 "}},
	}); st != http.StatusNoContent {
		t.Fatalf("guardar ubicación: %d %v", st, r)
	}
	st, v = c.pedir("POST", ruta+"/confirmar", map[string]any{"deposito_id": laPlata})
	if st != http.StatusOK || v["estado"] != "confirmado" {
		t.Fatalf("confirmar: %d %v", st, v)
	}
	// Confirmar no descuenta: el stock sale cuando el cliente retira.
	if got := stockEn(t, c, placaID, laPlata); got != 5 {
		t.Fatalf("stock en La Plata = %v, esperaba 5", got)
	}
	if v["reservado_hasta"] != nil {
		t.Fatalf("sin pagos no se reserva: %v", v["reservado_hasta"])
	}
	if u := v["lineas"].([]any)[0].(map[string]any)["ubicacion"]; u != "Estante B3" {
		t.Fatalf("ubicación en la venta = %v", u)
	}
	if v["retira"].(map[string]any)["nombre"] != "La Plata" || v["retira_aqui"] != false {
		t.Fatalf("se retira en La Plata: %v %v", v["retira"], v["retira_aqui"])
	}
	if st, _ := c.pedir("PUT", ruta, map[string]any{"cliente_id": cliID, "lineas": []any{
		map[string]any{"tipo": "producto", "producto_id": placaID, "cantidad": 1},
	}}); st != http.StatusConflict {
		t.Fatalf("editar confirmada: %d", st)
	}

	lineas := v["lineas"].([]any)
	placaLinea := lineas[0].(map[string]any)
	segundaLinea := lineas[1].(map[string]any)
	item := func(l map[string]any, cant float64) map[string]any {
		return map[string]any{"linea_id": l["id"], "tipo": l["tipo"], "cantidad": cant}
	}

	// Sin pagar no se retira.
	retirar := func(items ...map[string]any) (int, map[string]any) {
		return c.pedir("POST", ruta+"/retiros", map[string]any{"items": items})
	}
	if st, r := retirar(item(placaLinea, 1)); st != http.StatusUnprocessableEntity {
		t.Fatalf("retirar sin pagar: %d %v", st, r)
	}

	if st, _ := c.pedir("POST", ruta+"/pagos", map[string]any{"tipo": "sena", "monto": total + 1, "medio": "efectivo"}); st != http.StatusUnprocessableEntity {
		t.Fatalf("pago de más: %d", st)
	}
	st, v = c.pedir("POST", ruta+"/pagos", map[string]any{"tipo": "sena", "monto": 100000, "medio": "efectivo"})
	if st != http.StatusOK || v["reservado_hasta"] == nil {
		t.Fatalf("la seña reserva: %d %v", st, v["reservado_hasta"])
	}
	if p := productoEnStock(t, c, placaID); p["reservado_total"].(float64) != 4 || p["disponible"].(float64) != 1 {
		t.Fatalf("reservado = %v, disponible = %v", p["reservado_total"], p["disponible"])
	}
	st, v = c.pedir("POST", ruta+"/pagos", map[string]any{"tipo": "pago", "monto": total - 100000, "medio": "transferencia"})
	if st != http.StatusOK || v["pagada"] != true || v["saldo"].(float64) != 0 {
		t.Fatalf("pagar: %d %v", st, v)
	}

	// Vendida en la sucursal principal: ahí no se retira.
	if st, r := retirar(item(placaLinea, 1)); st != http.StatusUnprocessableEntity {
		t.Fatalf("retirar en otra sucursal: %d %v", st, r)
	}
	if st, _ := c.pedir("POST", "/api/v1/auth/sucursal", map[string]any{"sucursal_id": sucursalDeDeposito(t, pool, carpID, laPlata)}); st >= 300 {
		t.Fatalf("cambiar de sucursal: %d", st)
	}

	// Retira por partes.
	st, v = retirar(item(placaLinea, 2))
	if st != http.StatusOK || v["estado"] != "confirmado" || len(v["retiros"].([]any)) != 1 {
		t.Fatalf("primer retiro: %d %v", st, v)
	}
	// Se descuenta lo que se llevó; lo que falta sigue reservado.
	if p := productoEnStock(t, c, placaID); p["total"].(float64) != 3 || p["reservado_total"].(float64) != 2 {
		t.Fatalf("después del primer retiro: total %v, reservado %v", p["total"], p["reservado_total"])
	}
	if st, _ := retirar(item(placaLinea, 2)); st != http.StatusUnprocessableEntity {
		t.Fatalf("retirar más de lo que queda: %d", st)
	}
	if st, _ := c.pedir("POST", ruta+"/estado", map[string]any{"estado": "listo"}); st != http.StatusOK {
		t.Fatalf("estado listo: %d", st)
	}
	st, v = retirar(item(placaLinea, 1), item(segundaLinea, 1))
	if st != http.StatusOK || v["estado"] != "entregado" || v["todo_retirado"] != true {
		t.Fatalf("segundo retiro: %d %v", st, v)
	}
	if st, _ := retirar(item(placaLinea, 1)); st != http.StatusConflict {
		t.Fatalf("una boleta entregada no sirve de nuevo: %d", st)
	}
	if p := productoEnStock(t, c, placaID); p["total"].(float64) != 1 || p["reservado_total"].(float64) != 0 {
		t.Fatalf("después de entregar: total %v, reservado %v", p["total"], p["reservado_total"])
	}

	historial := v["historial"].([]any)
	if len(historial) != 4 { // presupuesto, confirmado, listo, entregado
		t.Errorf("historial = %v", historial)
	}

	st, lista := c.pedirLista("GET", "/api/v1/ventas?buscar=gómez")
	if st != http.StatusOK || len(lista) != 1 || lista[0]["retira"] != "La Plata" || lista[0]["saldo"].(float64) != 0 {
		t.Fatalf("listado: %d %v", st, lista)
	}
	if _, lista := c.pedirLista("GET", "/api/v1/ventas?estado=a_retirar"); len(lista) != 0 {
		t.Fatalf("no queda nada para retirar: %v", lista)
	}
	if _, lista := c.pedirLista("GET", "/api/v1/ventas?tipo=muebles"); len(lista) != 0 {
		t.Fatalf("no hay presupuestos de muebles: %v", lista)
	}

	// Productos vendidos: los dos renglones de la misma placa se suman.
	hoy := time.Now().In(zonaHoraria).Format("2006-01-02")
	st, vendidos := c.pedirLista("GET", "/api/v1/ventas/productos-vendidos?desde="+hoy+"&hasta="+hoy)
	if st != http.StatusOK || len(vendidos) != 1 || vendidos[0]["cantidad"].(float64) != 4 ||
		vendidos[0]["importe"].(float64) != total || vendidos[0]["ventas"].(float64) != 1 {
		t.Fatalf("productos vendidos: %d %v", st, vendidos)
	}
	if _, vendidos := c.pedirLista("GET", "/api/v1/ventas/productos-vendidos?desde=2020-01-01&hasta=2020-01-31"); len(vendidos) != 0 {
		t.Fatalf("otro período: %v", vendidos)
	}
}

func TestCancelarNoTocaStock(t *testing.T) {
	_, _, c, _, placaID, _, laPlata := prepararVenta(t)
	_, cli := c.pedir("POST", "/api/v1/clientes", map[string]any{"nombre": "Pedro"})
	_, v := c.pedir("POST", "/api/v1/ventas", map[string]any{
		"cliente_id": idDeJSON(cli),
		"lineas":     []any{map[string]any{"tipo": "producto", "producto_id": placaID, "cantidad": 4}},
	})
	ruta := fmt.Sprintf("/api/v1/ventas/%d", idDeJSON(v))
	c.pedir("POST", ruta+"/confirmar", map[string]any{"deposito_id": laPlata})
	if got := stockEn(t, c, placaID, laPlata); got != 5 {
		t.Fatalf("stock = %v", got)
	}
	c.pedir("POST", ruta+"/pagos", map[string]any{"tipo": "sena", "monto": 1000, "medio": "efectivo"})
	if st, _ := c.pedir("POST", ruta+"/cancelar", nil); st != http.StatusConflict {
		t.Fatalf("cancelar con seña: %d", st)
	}
	c.pedir("POST", ruta+"/pagos", map[string]any{"tipo": "devolucion", "monto": 1000, "medio": "efectivo"})
	st, v := c.pedir("POST", ruta+"/cancelar", nil)
	if st != http.StatusOK || v["estado"] != "cancelado" {
		t.Fatalf("cancelar: %d %v", st, v)
	}
	if p := productoEnStock(t, c, placaID); p["total"].(float64) != 5 || p["reservado_total"].(float64) != 0 {
		t.Fatalf("cancelada no reserva ni descuenta: %v %v", p["total"], p["reservado_total"])
	}
}

// Lo pagado y no retirado queda reservado 2 meses: no se le vende a otro, no
// se traspasa y no se usa en otra venta. Vencida la reserva, vuelve a estar libre.
func TestReservaEvitaSobreventa(t *testing.T) {
	pool, _, c, carpID, placaID, _, laPlata := prepararVenta(t) // 5 placas en La Plata
	_, cli := c.pedir("POST", "/api/v1/clientes", map[string]any{"nombre": "Pedro"})
	nueva := func(cant float64) string {
		_, v := c.pedir("POST", "/api/v1/ventas", map[string]any{
			"cliente_id": idDeJSON(cli),
			"lineas":     []any{map[string]any{"tipo": "producto", "producto_id": placaID, "cantidad": cant}},
		})
		return fmt.Sprintf("/api/v1/ventas/%d", idDeJSON(v))
	}
	a := nueva(4)
	c.pedir("POST", a+"/pagos", map[string]any{"tipo": "sena", "monto": 1000, "medio": "efectivo"}) // seña antes de confirmar
	st, v := c.pedir("POST", a+"/confirmar", map[string]any{"deposito_id": laPlata})
	if st != http.StatusOK || v["reservado_hasta"] == nil {
		t.Fatalf("confirmar con seña reserva: %d %v", st, v)
	}

	// Quedan 1 libre: otra venta de 2 no se confirma, una de 1 sí.
	b := nueva(2)
	if st, r := c.pedir("POST", b+"/confirmar", map[string]any{"deposito_id": laPlata}); st != http.StatusUnprocessableEntity {
		t.Fatalf("confirmar sobre lo reservado: %d %v", st, r)
	}
	// Tampoco se traspasa lo reservado.
	if st, r := c.pedir("POST", "/api/v1/stock/traspasos", map[string]any{
		"producto_id": placaID, "deposito_id": laPlata, "destino_id": depositoNuevo(t, pool, carpID, "Gonnet"), "cantidad": 2,
	}); st != http.StatusUnprocessableEntity {
		t.Fatalf("traspasar lo reservado: %d %v", st, r)
	}
	// Confirmada sin pagar no reserva, y la seña falla si ya no hay libre.
	cc := nueva(1)
	if st, r := c.pedir("POST", cc+"/confirmar", map[string]any{"deposito_id": laPlata}); st != http.StatusOK {
		t.Fatalf("confirmar 1: %d %v", st, r)
	}
	d := nueva(1)
	c.pedir("POST", d+"/confirmar", map[string]any{"deposito_id": laPlata})
	c.pedir("POST", d+"/pagos", map[string]any{"tipo": "sena", "monto": 100, "medio": "efectivo"}) // reserva la última
	if st, r := c.pedir("POST", cc+"/pagos", map[string]any{"tipo": "sena", "monto": 100, "medio": "efectivo"}); st != http.StatusUnprocessableEntity {
		t.Fatalf("señar sin stock libre: %d %v", st, r)
	}
	if p := productoEnStock(t, c, placaID); p["reservado_total"].(float64) != 5 || p["disponible"].(float64) != 0 {
		t.Fatalf("reservado %v disponible %v", p["reservado_total"], p["disponible"])
	}

	// La reserva de A vence: sus placas vuelven a estar libres.
	if err := store.EnCarpinteria(context.Background(), pool, carpID, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `UPDATE pedidos SET reservado_hasta = now() - interval '1 day' WHERE id = $1`, idDeRuta(a))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if st, r := c.pedir("POST", b+"/confirmar", map[string]any{"deposito_id": laPlata}); st != http.StatusOK {
		t.Fatalf("confirmar con la reserva vencida: %d %v", st, r)
	}
	if p := productoEnStock(t, c, placaID); p["reservado_total"].(float64) != 1 {
		t.Fatalf("reservado después de vencer = %v", p["reservado_total"])
	}
}

func idDeRuta(ruta string) int64 {
	var id int64
	fmt.Sscanf(ruta, "/api/v1/ventas/%d", &id)
	return id
}

func TestPermisosDeVentas(t *testing.T) {
	pool, _, c, carpID, placaID, _, laPlata := prepararVenta(t)
	_, cli := c.pedir("POST", "/api/v1/clientes", map[string]any{"nombre": "Ana"})
	_, v := c.pedir("POST", "/api/v1/ventas", map[string]any{
		"cliente_id": idDeJSON(cli),
		"lineas":     []any{map[string]any{"tipo": "producto", "producto_id": placaID, "cantidad": 1}},
	})
	ruta := fmt.Sprintf("/api/v1/ventas/%d", idDeJSON(v))
	c.pedir("POST", ruta+"/confirmar", map[string]any{"deposito_id": laPlata})

	agregarUsuario(t, pool, carpID, "operario@ejemplo.com", "Operario")
	agregarUsuario(t, pool, carpID, "deposito@ejemplo.com", "Depósito")
	op := nuevoCliente(t, c.srv)
	entrar(t, op, "operario@ejemplo.com")
	if st, _ := op.pedir("POST", "/api/v1/ventas", map[string]any{}); st != http.StatusForbidden {
		t.Errorf("operario crea ventas: %d", st)
	}
	if st, _ := op.pedir("POST", ruta+"/pagos", map[string]any{"tipo": "pago", "monto": 1, "medio": "efectivo"}); st != http.StatusForbidden {
		t.Errorf("operario cobra: %d", st)
	}
	if st, _ := op.pedir("POST", ruta+"/estado", map[string]any{"estado": "en_corte"}); st != http.StatusOK {
		t.Errorf("operario cambia estado: %d", st)
	}
	dep := nuevoCliente(t, c.srv)
	entrar(t, dep, "deposito@ejemplo.com")
	if st, _ := dep.pedir("GET", ruta, nil); st != http.StatusOK {
		t.Errorf("depósito ve la venta: %d", st)
	}
	if st, _ := dep.pedir("POST", ruta+"/confirmar", map[string]any{"deposito_id": laPlata}); st != http.StatusForbidden {
		t.Errorf("depósito confirma: %d", st)
	}
	if st, _ := op.pedir("POST", ruta+"/retiros", map[string]any{"items": []any{}}); st != http.StatusForbidden {
		t.Errorf("operario entrega: %d", st)
	}
}

func TestVentasAisladasYNumeracion(t *testing.T) {
	_, svc, c, _, placaID, _, _ := prepararVenta(t)
	_, cli := c.pedir("POST", "/api/v1/clientes", map[string]any{"nombre": "Ana"})
	cuerpo := map[string]any{
		"cliente_id": idDeJSON(cli),
		"lineas":     []any{map[string]any{"tipo": "producto", "producto_id": placaID, "cantidad": 1}},
	}

	// Diez presupuestos a la vez: números del 1 al 10 sin repetir.
	var wg sync.WaitGroup
	numeros := make(chan float64, 10)
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if st, v := c.pedir("POST", "/api/v1/ventas", cuerpo); st == http.StatusCreated {
				numeros <- v["numero"].(float64)
			}
		}()
	}
	wg.Wait()
	close(numeros)
	vistos := map[float64]bool{}
	for n := range numeros {
		vistos[n] = true
	}
	if len(vistos) != 10 {
		t.Fatalf("números = %v", vistos)
	}

	// Otra carpintería no ve estas ventas ni puede usar sus clientes.
	crear(t, svc, "Otra", "otra@ejemplo.com")
	o := nuevoCliente(t, c.srv)
	entrar(t, o, "otra@ejemplo.com")
	if st, _ := o.pedir("GET", "/api/v1/ventas/1", nil); st != http.StatusNotFound {
		t.Errorf("ve ventas ajenas: %d", st)
	}
	if _, lista := o.pedirLista("GET", "/api/v1/ventas"); len(lista) != 0 {
		t.Errorf("lista ventas ajenas: %v", lista)
	}
	if st, _ := o.pedir("POST", "/api/v1/ventas", map[string]any{
		"cliente_id": idDeJSON(cli),
		"lineas":     []any{},
	}); st != http.StatusUnprocessableEntity {
		t.Errorf("usa un cliente ajeno: %d", st)
	}
}
