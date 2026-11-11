// Package cortes es el módulo del optimizador de cortes y de los cortes
// optimizados guardados para el taller.
//
// El algoritmo es de tipo guillotina (cada corte va de lado a lado del
// rectángulo que se está cortando), que es lo que permite una escuadradora.
// Las medidas son en milímetros. El eje X es el largo de la placa, que es el
// sentido de la veta; el eje Y es el ancho. El origen (0,0) es la esquina
// inferior izquierda.
//
// Archivos del módulo: el modelo y sus reglas, service.go (la lógica de
// negocio), store.go (la interfaz Store con lo que el servicio necesita de la
// base), postgres_store.go (Store implementado con PostgreSQL) y handler.go
// (las rutas HTTP).
package cortes

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Cantos indica qué lados de la pieza llevan tapacanto.
// L1 y L2 son los lados largos; A1 y A2, los lados anchos.
type Cantos struct {
	L1 bool `json:"l1"`
	L2 bool `json:"l2"`
	A1 bool `json:"a1"`
	A2 bool `json:"a2"`
}

// Pieza es una pieza a cortar. La etiqueta (nombre) es opcional.
//
// Veta: si RespetaVeta es true, la veta de la pieza va horizontal (a lo
// largo de la pieza), salvo que VetaVertical diga que va a lo ancho. Si es
// false, la pieza se puede girar como convenga.
type Pieza struct {
	ID           string `json:"id"`
	Etiqueta     string `json:"etiqueta"`
	Largo        int    `json:"largo"`
	Ancho        int    `json:"ancho"`
	Cantidad     int    `json:"cantidad"`
	RespetaVeta  bool   `json:"respeta_veta"`
	VetaVertical bool   `json:"veta_vertical,omitempty"`
	Cantos       Cantos `json:"cantos"`
}

// puedeGirar dice si la pieza se puede colocar en esa orientación. La veta
// de la placa va a lo largo (eje X): una pieza con veta horizontal va sin
// girar y una con veta vertical va girada.
func puedeGirar(p Pieza, rotada bool) bool {
	return !p.RespetaVeta || rotada == p.VetaVertical
}

// Retazo es un sobrante de placa disponible para reutilizar.
type Retazo struct {
	ID    string `json:"id"`
	Largo int    `json:"largo"`
	Ancho int    `json:"ancho"`
}

// Parametros configura una optimización para un tipo de placa.
type Parametros struct {
	PlacaLargo int `json:"placa_largo"`
	PlacaAncho int `json:"placa_ancho"`
	// TieneVeta indica si la placa tiene veta. Si es false, todas las piezas
	// se pueden rotar aunque digan RespetaVeta.
	TieneVeta bool `json:"tiene_veta"`
	// Kerf es el espesor de la sierra.
	Kerf int `json:"kerf"`
	// Refilado se descuenta de cada borde de las placas enteras.
	Refilado int `json:"refilado"`
	// Retazos disponibles; se usan antes que las placas enteras.
	Retazos []Retazo `json:"retazos"`
	// Medida mínima para que un sobrante se proponga como retazo.
	SobranteMinLargo int `json:"sobrante_min_largo"`
	SobranteMinAncho int `json:"sobrante_min_ancho"`
}

// Rect es un rectángulo dentro de una placa.
type Rect struct {
	X     int `json:"x"`
	Y     int `json:"y"`
	Largo int `json:"largo"`
	Ancho int `json:"ancho"`
}

// Colocacion es una pieza ubicada en una placa.
type Colocacion struct {
	PiezaID  string `json:"pieza_id"`
	Etiqueta string `json:"etiqueta"`
	Rect
	Rotada bool `json:"rotada"`
}

// PlacaUsada es una placa entera o un retazo con sus piezas.
type PlacaUsada struct {
	Indice          int          `json:"indice"`
	RetazoID        string       `json:"retazo_id,omitempty"`
	Largo           int          `json:"largo"`
	Ancho           int          `json:"ancho"`
	Piezas          []Colocacion `json:"piezas"`
	Sobrantes       []Rect       `json:"sobrantes"`
	Aprovechamiento float64      `json:"aprovechamiento"` // porcentaje 0-100
	libres          []Rect
}

// Resultado es el plan de corte completo.
type Resultado struct {
	Placas          []PlacaUsada `json:"placas"`
	PlacasEnteras   int          `json:"placas_enteras"`
	RetazosUsados   int          `json:"retazos_usados"`
	NoColocadas     []Pieza      `json:"no_colocadas"`
	Aprovechamiento float64      `json:"aprovechamiento"`
}

type unidad struct {
	pieza  Pieza
	numero int // número de copia, para la etiqueta
}

// Optimizar arma un plan de corte para las piezas dadas.
func Optimizar(p Parametros, piezas []Pieza) (Resultado, error) {
	if p.PlacaLargo <= 0 || p.PlacaAncho <= 0 {
		return Resultado{}, errors.New("la placa debe tener largo y ancho mayores a cero")
	}
	if p.Kerf < 0 || p.Refilado < 0 {
		return Resultado{}, errors.New("kerf y refilado no pueden ser negativos")
	}
	if 2*p.Refilado >= p.PlacaLargo || 2*p.Refilado >= p.PlacaAncho {
		return Resultado{}, errors.New("el refilado es más grande que la placa")
	}

	var unidades []unidad
	for n, pz := range piezas {
		// El nombre es opcional: sin nombre, la pieza se llama por su número.
		pz.Etiqueta = strings.TrimSpace(pz.Etiqueta)
		if pz.Etiqueta == "" {
			pz.Etiqueta = fmt.Sprintf("Pieza %d", n+1)
		}
		if pz.Largo <= 0 || pz.Ancho <= 0 {
			return Resultado{}, fmt.Errorf("la pieza %q tiene medidas inválidas", pz.Etiqueta)
		}
		cant := pz.Cantidad
		if cant <= 0 {
			cant = 1
		}
		if !p.TieneVeta {
			pz.RespetaVeta = false
		}
		if !pz.RespetaVeta {
			pz.VetaVertical = false
		}
		for i := 1; i <= cant; i++ {
			unidades = append(unidades, unidad{pieza: pz, numero: i})
		}
	}
	// Primero las piezas más grandes: mejora mucho el aprovechamiento.
	sort.SliceStable(unidades, func(i, j int) bool {
		a, b := unidades[i].pieza, unidades[j].pieza
		if a.Largo*a.Ancho != b.Largo*b.Ancho {
			return a.Largo*a.Ancho > b.Largo*b.Ancho
		}
		return max(a.Largo, a.Ancho) > max(b.Largo, b.Ancho)
	})

	// Retazos de menor a mayor, para gastar primero los más chicos.
	retazos := append([]Retazo(nil), p.Retazos...)
	sort.SliceStable(retazos, func(i, j int) bool {
		return retazos[i].Largo*retazos[i].Ancho < retazos[j].Largo*retazos[j].Ancho
	})
	retazoUsado := make([]bool, len(retazos))

	var res Resultado
	placas := []*PlacaUsada{}

	// Los retazos se llenan primero: así se aprovechan aunque la pieza también
	// entre en una placa entera ya abierta, y la placa queda con sobrantes más grandes.
	abiertas := func(retazo bool) []*PlacaUsada {
		var out []*PlacaUsada
		for _, pl := range placas {
			if (pl.RetazoID != "") == retazo {
				out = append(out, pl)
			}
		}
		return out
	}
	for _, u := range unidades {
		if colocarEnAbiertas(abiertas(true), u, p.Kerf) {
			continue
		}
		// Abrir un retazo en el que la pieza quepa.
		var nueva *PlacaUsada
		for i, r := range retazos {
			if retazoUsado[i] {
				continue
			}
			if _, ok := orientacion(u.pieza, Rect{Largo: r.Largo, Ancho: r.Ancho}); ok {
				retazoUsado[i] = true
				nueva = &PlacaUsada{
					RetazoID: r.ID, Largo: r.Largo, Ancho: r.Ancho,
					libres: []Rect{{0, 0, r.Largo, r.Ancho}},
				}
				res.RetazosUsados++
				break
			}
		}
		if nueva == nil && colocarEnAbiertas(abiertas(false), u, p.Kerf) {
			continue
		}
		if nueva == nil {
			util := Rect{p.Refilado, p.Refilado, p.PlacaLargo - 2*p.Refilado, p.PlacaAncho - 2*p.Refilado}
			if _, ok := orientacion(u.pieza, util); !ok {
				res.NoColocadas = append(res.NoColocadas, u.pieza)
				continue
			}
			nueva = &PlacaUsada{Largo: p.PlacaLargo, Ancho: p.PlacaAncho, libres: []Rect{util}}
			res.PlacasEnteras++
		}
		nueva.Indice = len(placas) + 1
		placas = append(placas, nueva)
		if !colocarEnAbiertas([]*PlacaUsada{nueva}, u, p.Kerf) {
			return Resultado{}, errors.New("error interno: la pieza no entró en una placa nueva")
		}
	}

	var areaPiezas, areaPlacas int
	for _, pl := range placas {
		var area int
		for _, c := range pl.Piezas {
			area += c.Largo * c.Ancho
		}
		total := pl.Largo * pl.Ancho
		pl.Aprovechamiento = redondear(100 * float64(area) / float64(total))
		for _, l := range pl.libres {
			if esSobranteUtil(l, p.SobranteMinLargo, p.SobranteMinAncho) {
				pl.Sobrantes = append(pl.Sobrantes, l)
			}
		}
		areaPiezas += area
		areaPlacas += total
		res.Placas = append(res.Placas, *pl)
	}
	if areaPlacas > 0 {
		res.Aprovechamiento = redondear(100 * float64(areaPiezas) / float64(areaPlacas))
	}
	// Agrupar piezas no colocadas iguales.
	res.NoColocadas = agrupar(res.NoColocadas)
	return res, nil
}

// colocarEnAbiertas busca el mejor hueco libre entre las placas abiertas
// (el que deja menos área sobrante) y coloca la pieza ahí.
func colocarEnAbiertas(placas []*PlacaUsada, u unidad, kerf int) bool {
	mejorPlaca, mejorLibre := -1, -1
	var mejorRotada bool
	mejorSobra, mejorLado := -1, -1
	for pi, pl := range placas {
		for li, l := range pl.libres {
			for _, rot := range []bool{false, true} {
				if !puedeGirar(u.pieza, rot) {
					continue
				}
				w, h := dims(u.pieza, rot)
				if w > l.Largo || h > l.Ancho {
					continue
				}
				sobra := l.Largo*l.Ancho - w*h
				lado := min(l.Largo-w, l.Ancho-h)
				if mejorSobra < 0 || sobra < mejorSobra || (sobra == mejorSobra && lado < mejorLado) {
					mejorPlaca, mejorLibre, mejorRotada = pi, li, rot
					mejorSobra, mejorLado = sobra, lado
				}
			}
		}
	}
	if mejorPlaca < 0 {
		return false
	}
	pl := placas[mejorPlaca]
	l := pl.libres[mejorLibre]
	w, h := dims(u.pieza, mejorRotada)
	etiqueta := u.pieza.Etiqueta
	if u.pieza.Cantidad > 1 {
		etiqueta = fmt.Sprintf("%s (%d/%d)", u.pieza.Etiqueta, u.numero, u.pieza.Cantidad)
	}
	pl.Piezas = append(pl.Piezas, Colocacion{
		PiezaID: u.pieza.ID, Etiqueta: etiqueta,
		Rect: Rect{l.X, l.Y, w, h}, Rotada: mejorRotada,
	})
	// Reemplazar el hueco usado por los dos huecos que deja el corte.
	pl.libres = append(pl.libres[:mejorLibre], pl.libres[mejorLibre+1:]...)
	pl.libres = append(pl.libres, partir(l, w, h, kerf)...)
	return true
}

// partir corta el hueco libre l después de colocar una pieza de w x h en su
// esquina inferior izquierda. Hace primero el corte que deja el sobrante más
// grande entero (regla del eje sobrante más corto).
func partir(l Rect, w, h, kerf int) []Rect {
	restoX := l.Largo - w - kerf // a la derecha de la pieza
	restoY := l.Ancho - h - kerf // arriba de la pieza
	var derecha, arriba Rect
	if restoX < restoY {
		// Primer corte horizontal a lo largo de toda la placa.
		arriba = Rect{l.X, l.Y + h + kerf, l.Largo, restoY}
		derecha = Rect{l.X + w + kerf, l.Y, restoX, h}
	} else {
		// Primer corte vertical a todo el ancho.
		derecha = Rect{l.X + w + kerf, l.Y, restoX, l.Ancho}
		arriba = Rect{l.X, l.Y + h + kerf, w, restoY}
	}
	var out []Rect
	for _, r := range []Rect{derecha, arriba} {
		if r.Largo > 0 && r.Ancho > 0 {
			out = append(out, r)
		}
	}
	return out
}

func dims(p Pieza, rotada bool) (int, int) {
	if rotada {
		return p.Ancho, p.Largo
	}
	return p.Largo, p.Ancho
}

func orientacion(p Pieza, r Rect) (bool, bool) {
	if puedeGirar(p, false) && p.Largo <= r.Largo && p.Ancho <= r.Ancho {
		return false, true
	}
	if puedeGirar(p, true) && p.Ancho <= r.Largo && p.Largo <= r.Ancho {
		return true, true
	}
	return false, false
}

func esSobranteUtil(r Rect, minLargo, minAncho int) bool {
	if minLargo <= 0 && minAncho <= 0 {
		return false
	}
	a, b := max(r.Largo, r.Ancho), min(r.Largo, r.Ancho)
	return a >= max(minLargo, minAncho) && b >= min(minLargo, minAncho)
}

func agrupar(ps []Pieza) []Pieza {
	out := []Pieza{}
	idx := map[string]int{}
	for _, p := range ps {
		k := fmt.Sprintf("%s|%s|%d|%d", p.ID, p.Etiqueta, p.Largo, p.Ancho)
		if i, ok := idx[k]; ok {
			out[i].Cantidad++
			continue
		}
		p.Cantidad = 1
		idx[k] = len(out)
		out = append(out, p)
	}
	return out
}

func redondear(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}
