package api

import (
	"net/http"
	"strconv"
	"testing"
)

func TestPresupuestoDeMuebles(t *testing.T) {
	_, _, c, _, blanca, _, _ := prepararVenta(t)
	_, negra := c.pedir("POST", "/api/v1/productos", map[string]any{
		"tipo": "placa", "sku": "MEL-NE-18", "nombre": "Melamina Negra 18 mm", "tiene_veta": true,
		"largo_mm": 2600, "ancho_mm": 1830, "espesor_mm": 18, "costo": 60000, "precio_venta": 80000,
	})
	_, canto := c.pedir("POST", "/api/v1/productos", map[string]any{
		"tipo": "canto", "sku": "CAN-BL", "nombre": "Canto blanco", "unidad": "m", "costo": 100, "precio_venta": 200,
	})
	_, bisagra := c.pedir("POST", "/api/v1/productos", map[string]any{
		"tipo": "herraje", "sku": "BIS", "nombre": "Bisagra", "costo": 500, "precio_venta": 900,
	})
	for _, p := range []map[string]any{negra, canto, bisagra} {
		if p["id"] == nil {
			t.Fatalf("producto: %v", p)
		}
	}
	mueble := map[string]any{
		"nombre": "Bajo mesada", "cantidad": 2, "precio_unitario": 150000,
		"piezas": []map[string]any{
			{"nombre": "Lateral", "placa_id": blanca, "largo": 720, "ancho": 560, "cantidad": 2,
				"canto_id": idDeJSON(canto), "cantos": map[string]bool{"l1": true}},
			{"placa_id": blanca, "largo": 1164, "ancho": 560, "cantidad": 3},
			{"nombre": "Puerta", "placa_id": idDeJSON(negra), "largo": 716, "ancho": 396, "cantidad": 2, "respeta_veta": true},
		},
		"insumos": []map[string]any{{"producto_id": idDeJSON(bisagra), "cantidad": 4}},
	}
	_, cli := c.pedir("POST", "/api/v1/clientes", map[string]any{"nombre": "Carlos"})
	cuerpo := map[string]any{"cliente_id": idDeJSON(cli), "muebles": []any{mueble}}

	st, mat := c.pedir("POST", "/api/v1/presupuestos/calcular", cuerpo)
	if st != http.StatusOK {
		t.Fatalf("calcular: %d %v", st, mat)
	}
	if pl := mat["placas"].([]any); len(pl) != 2 {
		t.Fatalf("placas: %v", pl)
	}

	st, p := c.pedir("POST", "/api/v1/presupuestos", cuerpo)
	if st != http.StatusCreated {
		t.Fatalf("crear: %d %v", st, p)
	}
	venta := p["venta"].(map[string]any)
	id := idDeJSON(venta)
	if venta["tipo"] != "muebles" || p["materiales"] == nil {
		t.Fatalf("presupuesto: %v", p)
	}
	ms := p["muebles"].([]any)
	piezas := ms[0].(map[string]any)["piezas"].([]any)
	if len(piezas) != 3 || len(ms[0].(map[string]any)["insumos"].([]any)) != 1 {
		t.Fatalf("despiece guardado: %v", ms)
	}
	if piezas[0].(map[string]any)["cantos"].(map[string]any)["l1"] != true {
		t.Errorf("canto no guardado: %v", piezas[0])
	}

	// Los insumos del mueble no aparecen como productos sueltos de la venta.
	_, v := c.pedir("GET", "/api/v1/ventas/"+itoa(id), nil)
	for _, l := range v["lineas"].([]any) {
		if l.(map[string]any)["tipo"] != "mueble" {
			t.Errorf("línea que no es mueble: %v", l)
		}
	}

	// Editar: un mueble nuevo con piezas en otra placa.
	mueble["cantidad"] = 1
	cuerpo["muebles"] = []any{mueble, map[string]any{"nombre": "Estante", "cantidad": 1, "precio_unitario": 1000,
		"piezas": []map[string]any{{"placa_id": idDeJSON(negra), "largo": 800, "ancho": 300, "cantidad": 5}}}}
	st, p = c.pedir("PUT", "/api/v1/presupuestos/"+itoa(id), cuerpo)
	if st != http.StatusOK || len(p["muebles"].([]any)) != 2 {
		t.Fatalf("editar: %d %v", st, p)
	}

	// Fotos: se agregan a un mueble y siguen ahí después de editar.
	clave := p["muebles"].([]any)[0].(map[string]any)["clave"].(string)
	clave2 := p["muebles"].([]any)[1].(map[string]any)["clave"].(string)
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00")
	st, foto := c.pedir("POST", "/api/v1/presupuestos/"+itoa(id)+"/fotos", map[string]any{"mueble_clave": clave, "datos": png})
	if st != http.StatusCreated {
		t.Fatalf("subir foto: %d %v", st, foto)
	}
	_, foto2 := c.pedir("POST", "/api/v1/presupuestos/"+itoa(id)+"/fotos", map[string]any{"mueble_clave": clave2, "datos": png})
	if st, r := c.pedir("POST", "/api/v1/presupuestos/"+itoa(id)+"/fotos", map[string]any{"mueble_clave": clave, "datos": []byte("<svg onload=alert(1)>")}); st != http.StatusUnprocessableEntity {
		t.Errorf("foto que no es imagen: %d %v", st, r)
	}
	if st, _ := c.pedir("POST", "/api/v1/presupuestos/"+itoa(id)+"/fotos", map[string]any{"mueble_clave": "00000000-0000-0000-0000-000000000000", "datos": png}); st != http.StatusNotFound {
		t.Errorf("foto de un mueble que no existe: %d", st)
	}
	res, err := c.c.Get(c.srv.URL + "/api/v1/presupuestos/" + itoa(id) + "/fotos/" + itoa(idDeJSON(foto)))
	if err != nil || res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("ver foto: %v %v", err, res)
	}
	res.Body.Close()
	// Al editar se mantiene la clave del primer mueble; el segundo se saca y su foto se borra.
	cuerpo["muebles"] = []any{p["muebles"].([]any)[0]}
	st, p = c.pedir("PUT", "/api/v1/presupuestos/"+itoa(id), cuerpo)
	if st != http.StatusOK || p["muebles"].([]any)[0].(map[string]any)["clave"] != clave {
		t.Fatalf("editar con fotos: %d %v", st, p)
	}
	if fs := p["fotos"].([]any); len(fs) != 1 || idDeJSON(fs[0].(map[string]any)) != idDeJSON(foto) {
		t.Errorf("fotos después de editar: %v (la del mueble sacado era %v)", fs, foto2["id"])
	}
	if st, _ := c.pedir("DELETE", "/api/v1/presupuestos/"+itoa(id)+"/fotos/"+itoa(idDeJSON(foto)), nil); st != http.StatusNoContent {
		t.Errorf("borrar foto: %d", st)
	}

	// Errores de validación.
	malo := map[string]any{"cliente_id": idDeJSON(cli), "muebles": []any{map[string]any{"nombre": "X", "cantidad": 1,
		"piezas": []map[string]any{{"placa_id": idDeJSON(bisagra), "largo": 100, "ancho": 100, "cantidad": 1}}}}}
	if st, r := c.pedir("POST", "/api/v1/presupuestos", malo); st != http.StatusUnprocessableEntity {
		t.Errorf("placa que no es placa: %d %v", st, r)
	}
	// Un presupuesto de muebles no se edita como venta de mostrador, ni al revés.
	if st, _ := c.pedir("PUT", "/api/v1/ventas/"+itoa(id), map[string]any{"lineas": []any{}}); st == http.StatusOK {
		t.Error("se editó un presupuesto de muebles como mostrador")
	}
	_, mostrador := c.pedir("POST", "/api/v1/ventas", map[string]any{"cliente_id": idDeJSON(cli), "lineas": []any{map[string]any{"tipo": "producto", "producto_id": blanca, "cantidad": 1}}})
	if st, _ := c.pedir("GET", "/api/v1/presupuestos/"+itoa(idDeJSON(mostrador)), nil); st != http.StatusNotFound {
		t.Errorf("ver venta de mostrador como presupuesto: %d", st)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func TestConfirmarPresupuestoDescuentaMateriales(t *testing.T) {
	_, _, c, _, blanca, _, laPlata := prepararVenta(t) // 5 placas blancas en La Plata
	_, canto := c.pedir("POST", "/api/v1/productos", map[string]any{
		"tipo": "canto", "sku": "CAN-BL", "nombre": "Canto blanco", "unidad": "m", "costo": 100, "precio_venta": 200,
	})
	_, bisagra := c.pedir("POST", "/api/v1/productos", map[string]any{
		"tipo": "herraje", "sku": "BIS", "nombre": "Bisagra", "costo": 500, "precio_venta": 900,
	})
	ingreso := func(id int64, cant float64) {
		if st, r := c.pedir("POST", "/api/v1/stock/ingresos", map[string]any{"producto_id": id, "deposito_id": laPlata, "cantidad": cant}); st >= 300 {
			t.Fatalf("ingreso: %d %v", st, r)
		}
	}
	ingreso(idDeJSON(canto), 100)
	_, cli := c.pedir("POST", "/api/v1/clientes", map[string]any{"nombre": "Ana"})
	cuerpo := map[string]any{"cliente_id": idDeJSON(cli), "muebles": []any{map[string]any{
		"nombre": "Alacena", "cantidad": 2, "precio_unitario": 50000,
		"piezas": []map[string]any{{"placa_id": blanca, "largo": 700, "ancho": 300, "cantidad": 2,
			"canto_id": idDeJSON(canto), "cantos": map[string]bool{"l1": true}}},
		"insumos": []map[string]any{{"producto_id": idDeJSON(bisagra), "cantidad": 2}},
	}}}
	_, p := c.pedir("POST", "/api/v1/presupuestos", cuerpo)
	id := idDeJSON(p["venta"].(map[string]any))
	confirmar := "/api/v1/ventas/" + itoa(id) + "/confirmar"

	// Sin bisagras en el depósito no se confirma nada.
	if st, r := c.pedir("POST", confirmar, map[string]any{"deposito_id": laPlata}); st != http.StatusConflict && st != http.StatusUnprocessableEntity {
		t.Fatalf("confirmar sin bisagras: %d %v", st, r)
	} else if r["error"] == nil || stockEn(t, c, blanca, laPlata) != 5 {
		t.Fatalf("se descontó algo igual: %v", r)
	}
	ingreso(idDeJSON(bisagra), 10)
	if st, r := c.pedir("POST", confirmar, map[string]any{"deposito_id": laPlata}); st != http.StatusOK {
		t.Fatalf("confirmar: %d %v", st, r)
	}
	// 4 piezas de 700×300 entran en 1 placa; canto: 4 × (700+30) mm; 4 bisagras.
	if got := stockEn(t, c, blanca, laPlata); got != 4 {
		t.Errorf("placas = %v, esperaba 4", got)
	}
	if got := stockEn(t, c, idDeJSON(canto), laPlata); got != 100-2.92 {
		t.Errorf("canto = %v, esperaba 97.08", got)
	}
	if got := stockEn(t, c, idDeJSON(bisagra), laPlata); got != 6 {
		t.Errorf("bisagras = %v, esperaba 6", got)
	}
	// Cancelar devuelve todo.
	if st, r := c.pedir("POST", "/api/v1/ventas/"+itoa(id)+"/cancelar", nil); st != http.StatusOK {
		t.Fatalf("cancelar: %d %v", st, r)
	}
	if stockEn(t, c, blanca, laPlata) != 5 || stockEn(t, c, idDeJSON(canto), laPlata) != 100 || stockEn(t, c, idDeJSON(bisagra), laPlata) != 10 {
		t.Error("cancelar no devolvió los materiales")
	}
}

func TestRetazos(t *testing.T) {
	_, _, c, _, blanca, _, laPlata := prepararVenta(t) // 5 placas blancas en La Plata
	if st, _ := c.pedir("POST", "/api/v1/retazos", map[string]any{"placa_id": blanca, "deposito_id": laPlata, "largo": 3000, "ancho": 100}); st != http.StatusUnprocessableEntity {
		t.Errorf("retazo más grande que la placa: %d", st)
	}
	st, rz := c.pedir("POST", "/api/v1/retazos", map[string]any{"placa_id": blanca, "deposito_id": laPlata, "largo": 800, "ancho": 700, "nota": "estante de arriba"})
	if st != http.StatusCreated || rz["placa"] == nil {
		t.Fatalf("crear retazo: %d %v", st, rz)
	}
	disponibles := func() []map[string]any {
		_, l := c.pedirLista("GET", "/api/v1/retazos?placa_id="+itoa(blanca))
		return l
	}
	_, cli := c.pedir("POST", "/api/v1/clientes", map[string]any{"nombre": "Ana"})
	presupuesto := func(largo, ancho, cant int) int64 {
		_, p := c.pedir("POST", "/api/v1/presupuestos", map[string]any{"cliente_id": idDeJSON(cli), "muebles": []any{map[string]any{
			"nombre": "Repisa", "cantidad": 1, "precio_unitario": 1000,
			"piezas": []map[string]any{{"placa_id": blanca, "largo": largo, "ancho": ancho, "cantidad": cant}},
		}}})
		pl := p["materiales"].(map[string]any)["placas"].([]any)[0].(map[string]any)
		t.Logf("placas %v retazos %v", pl["cantidad"], pl["retazos"])
		return idDeJSON(p["venta"].(map[string]any))
	}

	// Dos piezas de 700×300 entran en el retazo de 800×700: no se usa ninguna placa.
	id := presupuesto(700, 300, 2)
	_, p := c.pedir("GET", "/api/v1/presupuestos/"+itoa(id), nil)
	pl := p["materiales"].(map[string]any)["placas"].([]any)[0].(map[string]any)
	if pl["cantidad"] != 0.0 || pl["retazos"] != 1.0 || pl["costo"] != 0.0 {
		t.Fatalf("con retazo: %v", pl)
	}
	if st, r := c.pedir("POST", "/api/v1/ventas/"+itoa(id)+"/confirmar", map[string]any{"deposito_id": laPlata}); st != http.StatusOK {
		t.Fatalf("confirmar: %d %v", st, r)
	}
	if got := stockEn(t, c, blanca, laPlata); got != 5 {
		t.Errorf("placas = %v, no se tenía que usar ninguna", got)
	}
	for _, x := range disponibles() {
		if idDeJSON(x) == idDeJSON(rz) {
			t.Errorf("el retazo sigue disponible: %v", x)
		}
	}
	// Después de confirmar se ve el plan guardado, con el retazo.
	_, p = c.pedir("GET", "/api/v1/presupuestos/"+itoa(id), nil)
	plan := p["materiales"].(map[string]any)["placas"].([]any)[0].(map[string]any)["plan"].(map[string]any)
	if plan["placas"].([]any)[0].(map[string]any)["retazo_id"] != itoa(idDeJSON(rz)) {
		t.Errorf("plan confirmado: %v", plan)
	}
	if st, _ := c.pedir("POST", "/api/v1/ventas/"+itoa(id)+"/cancelar", nil); st != http.StatusOK {
		t.Fatal("cancelar")
	}
	if l := disponibles(); len(l) != 1 {
		t.Fatalf("al cancelar el retazo vuelve: %v", l)
	}

	// Una pieza grande usa una placa entera y el sobrante queda como retazo.
	id = presupuesto(1500, 1000, 1)
	if st, r := c.pedir("POST", "/api/v1/ventas/"+itoa(id)+"/confirmar", map[string]any{"deposito_id": laPlata}); st != http.StatusOK {
		t.Fatalf("confirmar: %d %v", st, r)
	}
	nuevos := 0
	for _, x := range disponibles() {
		if x["origen"] != "" {
			nuevos++
		}
	}
	if nuevos == 0 || stockEn(t, c, blanca, laPlata) != 4 {
		t.Errorf("sobrantes: %d retazos nuevos, %v placas", nuevos, stockEn(t, c, blanca, laPlata))
	}
	if st, _ := c.pedir("POST", "/api/v1/ventas/"+itoa(id)+"/cancelar", nil); st != http.StatusOK {
		t.Fatal("cancelar")
	}
	if l := disponibles(); len(l) != 1 {
		t.Errorf("al cancelar se descartan los sobrantes: %v", l)
	}
	if st, _ := c.pedir("POST", "/api/v1/retazos/"+itoa(idDeJSON(rz))+"/descartar", nil); st != http.StatusNoContent || len(disponibles()) != 0 {
		t.Errorf("descartar: %d", st)
	}
}
