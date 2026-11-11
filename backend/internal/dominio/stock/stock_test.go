package stock

import (
	"errors"
	"strings"
	"testing"
)

func TestValidarPlaca(t *testing.T) {
	p := Producto{Tipo: TipoPlaca, Nombre: "Melamina Blanca 18", LargoMM: 2600, AnchoMM: 1830}
	if err := p.Validar(); err == nil {
		t.Fatal("una placa sin espesor no es válida")
	}
	p.EspesorMM = 18
	if err := p.Validar(); err != nil {
		t.Fatal(err)
	}
	p.RefiladoMM = 1000
	if err := p.Validar(); err == nil {
		t.Fatal("refilado imposible")
	}
}

func TestNormalizarLimpiaCamposDeOtroTipo(t *testing.T) {
	p := Producto{Tipo: TipoHerraje, Nombre: " Bisagra cazoleta ", LargoMM: 10, TieneVeta: true}
	p.Normalizar()
	if p.Nombre != "Bisagra cazoleta" || p.LargoMM != 0 || p.TieneVeta || p.Unidad != "unidad" {
		t.Fatalf("normalización incorrecta: %+v", p)
	}
}

func TestAjuste(t *testing.T) {
	op, err := Ajuste(1, 1, 7, "rotura")
	if err != nil {
		t.Fatal(err)
	}
	if nuevo, err := op.Nuevo(10); err != nil || nuevo != 7 {
		t.Fatalf("ajuste inesperado: %v %v", nuevo, err)
	}
	if _, err := op.Nuevo(7); err == nil {
		t.Fatal("si el contado es igual al actual no hay nada que ajustar")
	}
	if _, err := Ajuste(1, 1, 7, " "); err == nil {
		t.Fatal("el motivo es obligatorio")
	}
}

func TestTraspaso(t *testing.T) {
	if _, err := Traspaso(1, 2, 2, 5, ""); !errors.Is(err, ErrMismoDeposito) {
		t.Fatal("no se puede traspasar al mismo depósito")
	}
	ops, _ := Traspaso(1, 2, 3, 5, "")
	if ops[0].Cantidad != -5 || ops[1].Cantidad != 5 || ops[0].DepositoID != 2 || ops[1].DepositoID != 3 {
		t.Fatalf("traspaso inesperado: %+v", ops)
	}
	if _, err := ops[0].Nuevo(3); !errors.Is(err, ErrStockInsuficiente) {
		t.Fatal("no se puede sacar más de lo que hay")
	}
	if n, _ := ops[0].Nuevo(5); n != 0 {
		t.Fatal("debería quedar en cero")
	}
}

func TestStockBajo(t *testing.T) {
	if !EstaBajo(2, 5) || EstaBajo(5, 5) || EstaBajo(0, 0) {
		t.Fatal("regla de stock bajo incorrecta")
	}
}

func TestCategoria(t *testing.T) {
	c := Categoria{Nombre: "  Bisagras   y  guías ", Descripcion: " herrajes de puertas "}
	c.Normalizar()
	if c.Nombre != "Bisagras y guías" || c.Descripcion != "herrajes de puertas" {
		t.Fatalf("normalización incorrecta: %+v", c)
	}
	if err := c.Validar(); err != nil {
		t.Fatal(err)
	}
	vacia := Categoria{Nombre: "   "}
	vacia.Normalizar()
	if err := vacia.Validar(); err == nil {
		t.Fatal("una categoría sin nombre no es válida")
	}
	larga := Categoria{Nombre: "ñ" + strings.Repeat("a", 60)}
	if err := larga.Validar(); err == nil {
		t.Fatal("nombre de 61 letras no es válido")
	}
}

func TestNormalizarCategoriaDelProducto(t *testing.T) {
	cero := int64(0)
	p := Producto{Tipo: TipoHerraje, Nombre: "Bisagra", CategoriaID: &cero, Categoria: "inventada"}
	p.Normalizar()
	if p.CategoriaID != nil || p.Categoria != "" {
		t.Fatalf("categoría 0 tiene que quedar vacía: %+v", p)
	}
}
