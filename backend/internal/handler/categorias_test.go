package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func listarCategorias(t *testing.T, c *cliente) []map[string]any {
	t.Helper()
	res, err := c.c.Get(c.srv.URL + "/api/v1/categorias")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out []map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func listarCon(t *testing.T, c *cliente, query string) []map[string]any {
	t.Helper()
	res, err := c.c.Get(c.srv.URL + "/api/v1/productos?" + query)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out []map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return out
}

func TestCategorias(t *testing.T) {
	pool, svc, srv := preparar(t)
	id := crear(t, svc, "López", "juan@ejemplo.com")
	agregarUsuario(t, pool, id, "vendedor@ejemplo.com", "Vendedor")
	c := nuevoCliente(t, srv)
	entrar(t, c, "juan@ejemplo.com")

	if st, _ := c.pedir("POST", "/api/v1/categorias", map[string]any{"nombre": "  "}); st != http.StatusUnprocessableEntity {
		t.Fatalf("categoría sin nombre esperaba 422, obtuve %d", st)
	}
	st, bis := c.pedir("POST", "/api/v1/categorias", map[string]any{"nombre": "Bisagras", "descripcion": "Cazoletas y codos"})
	if st != http.StatusCreated {
		t.Fatalf("crear categoría: %d %v", st, bis)
	}
	if st, _ := c.pedir("POST", "/api/v1/categorias", map[string]any{"nombre": "bisagras"}); st != http.StatusConflict {
		t.Fatalf("nombre repetido (sin importar mayúsculas) esperaba 409, obtuve %d", st)
	}
	_, guias := c.pedir("POST", "/api/v1/categorias", map[string]any{"nombre": "Guías"})
	bisID := int64(bis["id"].(float64))

	// Producto con categoría y otro sin.
	bisagra := map[string]any{"tipo": "herraje", "nombre": "Bisagra cazoleta 35 mm", "categoria_id": bisID}
	st, prod := c.pedir("POST", "/api/v1/productos", bisagra)
	if st != http.StatusCreated {
		t.Fatalf("crear producto con categoría: %d %v", st, prod)
	}
	c.pedir("POST", "/api/v1/productos", placa)
	ps := listarCon(t, c, fmt.Sprintf("categoria=%d", bisID))
	if len(ps) != 1 || ps[0]["categoria"] != "Bisagras" {
		t.Fatalf("filtro por categoría: %v", ps)
	}
	if ps := listarCon(t, c, "categoria=sin"); len(ps) != 1 || ps[0]["nombre"] != placa["nombre"] {
		t.Fatalf("filtro sin categoría: %v", ps)
	}
	cs := listarCategorias(t, c)
	if len(cs) != 2 || cs[0]["nombre"] != "Bisagras" || cs[0]["productos"].(float64) != 1 || cs[1]["productos"].(float64) != 0 {
		t.Fatalf("categorías inesperadas: %v", cs)
	}

	// Renombrar y pasar el producto a otra categoría.
	if st, _ := c.pedir("PUT", fmt.Sprintf("/api/v1/categorias/%d", bisID), map[string]any{"nombre": "Guías"}); st != http.StatusConflict {
		t.Fatalf("renombrar a un nombre existente esperaba 409, obtuve %d", st)
	}
	if st, _ := c.pedir("PUT", fmt.Sprintf("/api/v1/categorias/%d", bisID), map[string]any{"nombre": "Bisagras y codos"}); st != http.StatusOK {
		t.Fatalf("renombrar: %d", st)
	}
	prod["categoria_id"] = guias["id"]
	if st, p := c.pedir("PUT", fmt.Sprintf("/api/v1/productos/%v", prod["id"]), prod); st != http.StatusOK {
		t.Fatalf("cambiar categoría: %d %v", st, p)
	}
	prod["categoria_id"] = bisID

	// Otra carpintería no ve las categorías ni puede usarlas.
	crear(t, svc, "Otra", "otra@ejemplo.com")
	o := nuevoCliente(t, srv)
	entrar(t, o, "otra@ejemplo.com")
	if len(listarCategorias(t, o)) != 0 {
		t.Fatal("la otra carpintería no debería ver categorías ajenas")
	}
	ajeno := map[string]any{"tipo": "herraje", "nombre": "Bisagra", "categoria_id": bisID}
	if st, _ := o.pedir("POST", "/api/v1/productos", ajeno); st != http.StatusUnprocessableEntity {
		t.Fatalf("categoría ajena esperaba 422, obtuve %d", st)
	}
	if st, _ := o.pedir("DELETE", fmt.Sprintf("/api/v1/categorias/%d", bisID), nil); st != http.StatusNotFound {
		t.Fatalf("borrar categoría ajena esperaba 404, obtuve %d", st)
	}

	// El vendedor ve las categorías pero no las toca.
	v := nuevoCliente(t, srv)
	entrar(t, v, "vendedor@ejemplo.com")
	if len(listarCategorias(t, v)) != 2 {
		t.Fatal("el vendedor sí ve categorías")
	}
	if st, _ := v.pedir("POST", "/api/v1/categorias", map[string]any{"nombre": "Patas"}); st != http.StatusForbidden {
		t.Fatalf("el vendedor no crea categorías, obtuve %d", st)
	}

	// Borrar una categoría deja a sus productos sin categoría.
	if st, _ := c.pedir("DELETE", fmt.Sprintf("/api/v1/categorias/%v", guias["id"]), nil); st != http.StatusNoContent {
		t.Fatalf("borrar categoría: %d", st)
	}
	if ps := listarCon(t, c, "categoria=sin"); len(ps) != 2 {
		t.Fatalf("los productos deberían quedar sin categoría: %v", ps)
	}
}
