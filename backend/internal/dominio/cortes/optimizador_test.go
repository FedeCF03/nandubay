package cortes

import (
	"math/rand"
	"testing"
)

func base() Parametros {
	return Parametros{PlacaLargo: 2600, PlacaAncho: 1830, TieneVeta: true, Kerf: 3}
}

func TestKerfSeRespeta(t *testing.T) {
	// Dos piezas de 1300 no entran a lo largo por los 3 mm de sierra.
	p := base()
	p.PlacaAncho = 500
	res, err := Optimizar(p, []Pieza{{ID: "a", Etiqueta: "A", Largo: 1300, Ancho: 500, Cantidad: 2, RespetaVeta: true}})
	if err != nil {
		t.Fatal(err)
	}
	if res.PlacasEnteras != 2 {
		t.Fatalf("esperaba 2 placas, obtuve %d", res.PlacasEnteras)
	}
	// Con 1298 sí entran las dos en una.
	res, _ = Optimizar(p, []Pieza{{ID: "a", Etiqueta: "A", Largo: 1298, Ancho: 500, Cantidad: 2, RespetaVeta: true}})
	if res.PlacasEnteras != 1 {
		t.Fatalf("esperaba 1 placa, obtuve %d", res.PlacasEnteras)
	}
	if x := res.Placas[0].Piezas[1].X; x != 1301 {
		t.Fatalf("la segunda pieza debería empezar en 1301, empieza en %d", x)
	}
}

func TestVetaNoRota(t *testing.T) {
	pz := []Pieza{{ID: "a", Etiqueta: "Puerta", Largo: 1000, Ancho: 2000, Cantidad: 1, RespetaVeta: true}}
	res, _ := Optimizar(base(), pz)
	if len(res.NoColocadas) != 1 {
		t.Fatal("una pieza con veta de 2000 de ancho no debería entrar en 1830")
	}
	pz[0].RespetaVeta = false
	res, _ = Optimizar(base(), pz)
	if len(res.NoColocadas) != 0 || !res.Placas[0].Piezas[0].Rotada {
		t.Fatal("sin veta la pieza debería entrar rotada")
	}
}

func TestPlacaSinVetaPermiteRotar(t *testing.T) {
	p := base()
	p.TieneVeta = false
	res, _ := Optimizar(p, []Pieza{{ID: "a", Etiqueta: "A", Largo: 1000, Ancho: 2000, RespetaVeta: true}})
	if len(res.NoColocadas) != 0 {
		t.Fatal("en placa sin veta se debería poder rotar")
	}
}

func TestRefilado(t *testing.T) {
	p := base()
	p.Refilado = 10
	res, _ := Optimizar(p, []Pieza{{ID: "a", Etiqueta: "A", Largo: 2590, Ancho: 100, RespetaVeta: true}})
	if len(res.NoColocadas) != 1 {
		t.Fatal("con 10 mm de refilado por lado entran como máximo 2580")
	}
	res, _ = Optimizar(p, []Pieza{{ID: "a", Etiqueta: "A", Largo: 2580, Ancho: 100, RespetaVeta: true}})
	if c := res.Placas[0].Piezas[0]; c.X != 10 || c.Y != 10 {
		t.Fatalf("la pieza debería empezar en (10,10), empieza en (%d,%d)", c.X, c.Y)
	}
}

func TestUsaRetazosPrimero(t *testing.T) {
	p := base()
	p.Retazos = []Retazo{{ID: "r-grande", Largo: 1500, Ancho: 800}, {ID: "r-chico", Largo: 600, Ancho: 400}}
	res, _ := Optimizar(p, []Pieza{{ID: "a", Etiqueta: "Estante", Largo: 550, Ancho: 350, RespetaVeta: true}})
	if res.PlacasEnteras != 0 || res.RetazosUsados != 1 || res.Placas[0].RetazoID != "r-chico" {
		t.Fatalf("debería usar el retazo chico: %+v", res)
	}
}

func TestSinSuperposicionesAleatorio(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for caso := 0; caso < 50; caso++ {
		p := base()
		p.Refilado = r.Intn(15)
		p.TieneVeta = r.Intn(2) == 0
		var pz []Pieza
		for i := 0; i < 1+r.Intn(25); i++ {
			pz = append(pz, Pieza{
				ID: string(rune('a' + i)), Etiqueta: "P", Largo: 50 + r.Intn(2000), Ancho: 50 + r.Intn(1200),
				Cantidad: 1 + r.Intn(4), RespetaVeta: r.Intn(2) == 0,
			})
		}
		res, err := Optimizar(p, pz)
		if err != nil {
			t.Fatal(err)
		}
		total := 0
		for _, x := range pz {
			total += x.Cantidad
		}
		colocadas := 0
		for _, pl := range res.Placas {
			colocadas += len(pl.Piezas)
			for i, a := range pl.Piezas {
				if a.X < 0 || a.Y < 0 || a.X+a.Largo > pl.Largo || a.Y+a.Ancho > pl.Ancho {
					t.Fatalf("caso %d: pieza fuera de la placa: %+v", caso, a)
				}
				for _, b := range pl.Piezas[i+1:] {
					// Entre dos piezas tiene que haber al menos el kerf.
					sep := a.X+a.Largo+p.Kerf <= b.X || b.X+b.Largo+p.Kerf <= a.X ||
						a.Y+a.Ancho+p.Kerf <= b.Y || b.Y+b.Ancho+p.Kerf <= a.Y
					if !sep {
						t.Fatalf("caso %d: piezas superpuestas o sin kerf: %+v %+v", caso, a, b)
					}
				}
			}
		}
		for _, n := range res.NoColocadas {
			colocadas += n.Cantidad
		}
		if colocadas != total {
			t.Fatalf("caso %d: se perdieron piezas: %d de %d", caso, colocadas, total)
		}
	}
}

func TestMetrosCanto(t *testing.T) {
	pz := []Pieza{
		{Largo: 720, Ancho: 560, Cantidad: 2, Cantos: Cantos{L1: true}},                               // 2 x 720
		{Largo: 800, Ancho: 400, Cantidad: 1, Cantos: Cantos{L1: true, L2: true, A1: true, A2: true}}, // 2400
	}
	if m := MetrosCanto(pz, 0); m != 3.84 {
		t.Fatalf("esperaba 3.84 m, obtuve %v", m)
	}
	if m := MetrosCanto(pz, 20); m != 3.96 {
		t.Fatalf("esperaba 3.96 m con merma, obtuve %v", m)
	}
}

func TestNombreOpcional(t *testing.T) {
	res, err := Optimizar(base(), []Pieza{
		{ID: "a", Etiqueta: "Lateral", Largo: 720, Ancho: 560, Cantidad: 1},
		{ID: "b", Largo: 500, Ancho: 300, Cantidad: 2},
		{ID: "c", Etiqueta: "   ", Largo: 400, Ancho: 200, Cantidad: 1},
		{ID: "d", Largo: 3000, Ancho: 100, Cantidad: 1}, // no entra
	})
	if err != nil {
		t.Fatal(err)
	}
	nombres := map[string]string{}
	for _, c := range res.Placas[0].Piezas {
		nombres[c.PiezaID+c.Etiqueta] = c.Etiqueta
	}
	for _, n := range []string{"aLateral", "bPieza 2 (1/2)", "bPieza 2 (2/2)", "cPieza 3"} {
		if _, ok := nombres[n]; !ok {
			t.Errorf("falta %q en %v", n, nombres)
		}
	}
	if len(res.NoColocadas) != 1 || res.NoColocadas[0].Etiqueta != "Pieza 4" {
		t.Errorf("no colocadas = %+v", res.NoColocadas)
	}
}
