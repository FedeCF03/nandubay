package muebles

import (
	"errors"
	"testing"

	"github.com/FedeCF03/nandubay/backend/internal/dominio/cortes"
	"github.com/FedeCF03/nandubay/backend/internal/dominio/stock"
)

var productos = map[int64]stock.Producto{
	1: {ID: 1, Tipo: stock.TipoPlaca, Nombre: "Melamina blanca", LargoMM: 2600, AnchoMM: 1830, TieneVeta: false, Costo: 50000},
	2: {ID: 2, Tipo: stock.TipoPlaca, Nombre: "Melamina negra", LargoMM: 2600, AnchoMM: 1830, TieneVeta: true, Costo: 60000},
	3: {ID: 3, Tipo: stock.TipoCanto, Nombre: "Canto blanco", Costo: 100},
	4: {ID: 4, Tipo: stock.TipoHerraje, Nombre: "Bisagra", Costo: 500},
	5: {ID: 5, Tipo: stock.TipoHerraje, Nombre: "Pata regulable", Costo: 300},
}

func bajoMesada() Mueble {
	return Mueble{
		Nombre: "Bajo mesada", Cantidad: 1, Precio: 100000,
		Piezas: []Pieza{
			{Nombre: "Lateral", PlacaID: 1, Largo: 720, Ancho: 560, Cantidad: 2, CantoID: 3, Cantos: cortes.Cantos{L1: true}},
			{PlacaID: 1, Largo: 1164, Ancho: 560, Cantidad: 2}, // piso y techo, sin nombre
			{Nombre: "Puerta", PlacaID: 2, Largo: 716, Ancho: 396, Cantidad: 3, RespetaVeta: true, CantoID: 3, // 5 blancas + 3 negras
				Cantos: cortes.Cantos{L1: true, L2: true, A1: true, A2: true}},
		},
		Insumos: []Insumo{{ProductoID: 4, Cantidad: 6}, {ProductoID: 5, Cantidad: 4}},
	}
}

func TestValidar(t *testing.T) {
	ms := []Mueble{bajoMesada()}
	if err := Validar(ms, productos); err != nil {
		t.Fatal(err)
	}
	malos := []func(m *Mueble){
		func(m *Mueble) { m.Nombre = " " },
		func(m *Mueble) { m.Cantidad = 0 },
		func(m *Mueble) { m.Piezas[0].PlacaID = 3 },                          // un canto no es placa
		func(m *Mueble) { m.Piezas[0].Largo = 0 },                            // sin medida
		func(m *Mueble) { m.Piezas[0].Largo = 3000 },                         // no entra
		func(m *Mueble) { m.Piezas[0].CantoID = 0 },                          // lleva canto pero no dice cuál
		func(m *Mueble) { m.Insumos[0].ProductoID = 1 },                      // una placa como insumo
		func(m *Mueble) { m.Insumos[0].Cantidad = 1.5 },                      // bisagras enteras
		func(m *Mueble) { m.Piezas[2].Largo, m.Piezas[2].Ancho = 300, 1900 }, // con veta no se rota
	}
	for i, f := range malos {
		m := bajoMesada()
		f(&m)
		var ev ErrValidacion
		if err := Validar([]Mueble{m}, productos); !errors.As(err, &ev) {
			t.Errorf("caso %d: esperaba error de validación, salió %v", i, err)
		}
	}
	// Una pieza sin canto no necesita elegir canto.
	m := bajoMesada()
	m.Piezas[1].CantoID = 99
	if err := Validar([]Mueble{m}, productos); err != nil || m.Piezas[1].CantoID != 0 {
		t.Errorf("pieza sin canto: %v %d", err, m.Piezas[1].CantoID)
	}
	// Sin veta la pieza sí se puede rotar.
	m = bajoMesada()
	m.Piezas[0].Largo, m.Piezas[0].Ancho = 300, 1900
	if err := Validar([]Mueble{m}, productos); err != nil {
		t.Error(err)
	}
}

func TestCalcularVariasPlacas(t *testing.T) {
	a := bajoMesada()
	b := bajoMesada()
	b.Nombre, b.Cantidad = "Alacena", 2
	ms := []Mueble{a, b}
	if err := Validar(ms, productos); err != nil {
		t.Fatal(err)
	}
	mat, err := Calcular(ms, productos, 3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(mat.Placas) != 2 || mat.Placas[0].ProductoID != 1 || mat.Placas[1].ProductoID != 2 {
		t.Fatalf("una placa calculada por material: %+v", mat.Placas)
	}
	// Las piezas de los dos muebles se optimizan juntas.
	piezasBlancas := 0
	for _, pl := range mat.Placas[0].Plan.Placas {
		piezasBlancas += len(pl.Piezas)
	}
	if piezasBlancas != (2+2)*3 {
		t.Errorf("piezas blancas colocadas = %d, esperaba 12", piezasBlancas)
	}
	if mat.Placas[1].Cantidad != 1 || mat.Placas[1].Costo != 60000 {
		t.Errorf("negra: %+v", mat.Placas[1])
	}
	// Canto: laterales 1 lado de 720 y puertas 4 lados, por 3 muebles, con 30 mm por tramo.
	esperado := float64(3) * (2*(720+30) + 3*(2*(716+30)+2*(396+30))) / 1000
	if len(mat.Cantos) != 1 || mat.Cantos[0].Cantidad != redondear(esperado) {
		t.Errorf("canto = %+v, esperaba %v m", mat.Cantos, esperado)
	}
	if len(mat.Insumos) != 2 || mat.Insumos[0].Cantidad != 18 || mat.Insumos[1].Cantidad != 12 {
		t.Errorf("insumos = %+v", mat.Insumos)
	}
	// El costo de cada mueble repartido suma el total.
	suma := mat.CostoPorMueble[0] + 2*mat.CostoPorMueble[1]
	if d := suma - mat.Costo; d > 2 || d < -2 { // redondeo a centavos
		t.Errorf("costo por mueble %v suma %v, total %v", mat.CostoPorMueble, suma, mat.Costo)
	}
	if mat.CostoPorMueble[0] != mat.CostoPorMueble[1] {
		t.Errorf("dos muebles iguales cuestan distinto: %v", mat.CostoPorMueble)
	}
	// Las etiquetas dicen de qué mueble es cada pieza.
	if et := mat.Placas[1].Plan.Placas[0].Piezas[0].Etiqueta; et == "" {
		t.Error("pieza sin etiqueta")
	}
}

func TestProductosUsados(t *testing.T) {
	got := ProductosUsados([]Mueble{bajoMesada()})
	if len(got) != 5 {
		t.Errorf("productos usados = %v", got)
	}
}

func TestPrecioDe(t *testing.T) {
	casos := map[float64]float64{0: 0, 100: 300, 1000: 2700, 50000: 135000, 12345: 33400}
	for costo, quiero := range casos {
		if got := PrecioDe(costo); got != quiero {
			t.Errorf("PrecioDe(%v) = %v, quería %v", costo, got, quiero)
		}
	}
}
