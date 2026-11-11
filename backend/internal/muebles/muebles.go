// Package muebles tiene las reglas de los presupuestos de muebles: cada
// mueble tiene sus piezas (cada una de la placa que corresponda) e insumos
// (bisagras, patas, guías…). De ahí se calculan las placas, los metros de
// canto y el costo de materiales, optimizando juntas todas las piezas del
// presupuesto que van en la misma placa. Es solo modelo: lo usan los
// presupuestos de ventas.
package muebles

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/FedeCF03/nandubay/backend/internal/cortes"
	"github.com/FedeCF03/nandubay/backend/internal/stock"
)

// ErrValidacion es un error en los datos cargados.
type ErrValidacion struct{ Campo, Mensaje string }

func (e ErrValidacion) Error() string { return e.Mensaje }

// Pieza es una pieza a cortar del mueble, de una placa del stock.
type Pieza struct {
	Nombre      string `json:"nombre"`
	PlacaID     int64  `json:"placa_id"`
	Largo       int    `json:"largo"`
	Ancho       int    `json:"ancho"`
	Cantidad    int    `json:"cantidad"`
	RespetaVeta bool   `json:"respeta_veta"`
	// VetaVertical: la veta va a lo ancho de la pieza (ver cortes.Pieza).
	VetaVertical bool          `json:"veta_vertical,omitempty"`
	Cantos       cortes.Cantos `json:"cantos"`
	CantoID      int64         `json:"canto_id,omitempty"`
}

// Insumo es un producto del stock que lleva el mueble, por unidad de mueble.
type Insumo struct {
	ProductoID int64   `json:"producto_id"`
	Cantidad   float64 `json:"cantidad"`
}

// Mueble de un presupuesto. Las cantidades de piezas e insumos son por cada
// mueble; Cantidad dice cuántos muebles iguales se hacen.
type Mueble struct {
	// Clave identifica al mueble entre ediciones del presupuesto (para sus
	// fotos). Si viene vacía o repetida se crea una nueva al guardar.
	Clave         string   `json:"clave,omitempty"`
	Nombre        string   `json:"nombre"`
	AnchoMM       int      `json:"ancho_mm"`
	AltoMM        int      `json:"alto_mm"`
	ProfundidadMM int      `json:"profundidad_mm"`
	Cantidad      int      `json:"cantidad"`
	Precio        float64  `json:"precio_unitario"`
	Piezas        []Pieza  `json:"piezas"`
	Insumos       []Insumo `json:"insumos"`
}

var esUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Validar limpia y controla los muebles contra los productos del stock.
func Validar(ms []Mueble, productos map[int64]stock.Producto) error {
	if len(ms) == 0 {
		return ErrValidacion{"muebles", "agregá al menos un mueble"}
	}
	if len(ms) > 100 {
		return ErrValidacion{"muebles", "demasiados muebles en un presupuesto"}
	}
	claves := map[string]bool{}
	for i := range ms {
		m := &ms[i]
		n := i + 1
		m.Clave = strings.ToLower(strings.TrimSpace(m.Clave))
		if !esUUID.MatchString(m.Clave) || claves[m.Clave] {
			m.Clave = ""
		}
		claves[m.Clave] = true
		m.Nombre = strings.TrimSpace(m.Nombre)
		if m.Nombre == "" {
			return ErrValidacion{fmt.Sprintf("muebles.%d.nombre", i), fmt.Sprintf("mueble %d: poné qué mueble es", n)}
		}
		if len(m.Nombre) > 200 {
			return ErrValidacion{fmt.Sprintf("muebles.%d.nombre", i), fmt.Sprintf("mueble %d: el nombre es demasiado largo", n)}
		}
		if m.Cantidad <= 0 || m.Cantidad > 1000 {
			return ErrValidacion{fmt.Sprintf("muebles.%d.cantidad", i), fmt.Sprintf("%s: la cantidad tiene que ser mayor a cero", m.Nombre)}
		}
		if m.Precio < 0 || m.Precio > 1e10 {
			return ErrValidacion{fmt.Sprintf("muebles.%d.precio_unitario", i), fmt.Sprintf("%s: el precio no es válido", m.Nombre)}
		}
		for _, d := range []int{m.AnchoMM, m.AltoMM, m.ProfundidadMM} {
			if d < 0 || d > 20000 {
				return ErrValidacion{fmt.Sprintf("muebles.%d.ancho_mm", i), fmt.Sprintf("%s: las medidas no son válidas", m.Nombre)}
			}
		}
		if len(m.Piezas) > 500 || len(m.Insumos) > 200 {
			return ErrValidacion{fmt.Sprintf("muebles.%d", i), fmt.Sprintf("%s: demasiados renglones", m.Nombre)}
		}
		for j := range m.Piezas {
			p := &m.Piezas[j]
			p.Nombre = strings.TrimSpace(p.Nombre)
			que := fmt.Sprintf("%s, pieza %d", m.Nombre, j+1)
			campo := fmt.Sprintf("muebles.%d.piezas.%d", i, j)
			placa, ok := productos[p.PlacaID]
			if !ok || placa.Tipo != stock.TipoPlaca {
				return ErrValidacion{campo + ".placa_id", que + ": elegí la placa"}
			}
			if p.Largo <= 0 || p.Ancho <= 0 || p.Cantidad <= 0 || p.Cantidad > 1000 {
				return ErrValidacion{campo, que + ": cargá largo, ancho y cantidad"}
			}
			if !p.RespetaVeta {
				p.VetaVertical = false
			}
			if !entra(p.Largo, p.Ancho, p.RespetaVeta && placa.TieneVeta, p.VetaVertical, placa) {
				return ErrValidacion{campo, fmt.Sprintf("%s: %d × %d no entra en %s", que, p.Largo, p.Ancho, placa.Nombre)}
			}
			conCanto := p.Cantos.L1 || p.Cantos.L2 || p.Cantos.A1 || p.Cantos.A2
			if !conCanto {
				p.CantoID = 0
			} else if c, ok := productos[p.CantoID]; !ok || c.Tipo != stock.TipoCanto {
				return ErrValidacion{campo + ".canto_id", que + ": elegí qué canto lleva"}
			}
		}
		for j, ins := range m.Insumos {
			pr, ok := productos[ins.ProductoID]
			campo := fmt.Sprintf("muebles.%d.insumos.%d", i, j)
			if !ok || pr.Tipo == stock.TipoPlaca {
				return ErrValidacion{campo, fmt.Sprintf("%s, insumo %d: elegí un producto del stock (las placas van en las piezas)", m.Nombre, j+1)}
			}
			if ins.Cantidad <= 0 || ins.Cantidad > 1e6 {
				return ErrValidacion{campo, fmt.Sprintf("%s: la cantidad de %s tiene que ser mayor a cero", m.Nombre, pr.Nombre)}
			}
			if stock.UnidadDe(pr.Tipo) != "metro" && ins.Cantidad != math.Trunc(ins.Cantidad) {
				return ErrValidacion{campo, fmt.Sprintf("%s: %s va por unidad entera", m.Nombre, pr.Nombre)}
			}
		}
	}
	return nil
}

// util es la medida aprovechable de la placa, sin el refilado.
func util(p stock.Producto) (int, int) {
	return p.LargoMM - 2*p.RefiladoMM, p.AnchoMM - 2*p.RefiladoMM
}

func entra(largo, ancho int, respetaVeta, vertical bool, placa stock.Producto) bool {
	L, A := util(placa)
	if (!respetaVeta || !vertical) && largo <= L && ancho <= A {
		return true
	}
	return (!respetaVeta || vertical) && ancho <= L && largo <= A
}

// PlacaCalculada es cuántas placas de un material hacen falta y cómo se cortan.
type PlacaCalculada struct {
	ProductoID      int64            `json:"producto_id"`
	Nombre          string           `json:"nombre"`
	Cantidad        int              `json:"cantidad"` // placas enteras
	Retazos         int              `json:"retazos"`  // retazos usados
	Aprovechamiento float64          `json:"aprovechamiento"`
	CostoUnitario   float64          `json:"costo_unitario"`
	Costo           float64          `json:"costo"`
	Plan            cortes.Resultado `json:"plan"`
	// Con qué se optimizó, para volver a optimizar con retazos al cortar.
	Parametros cortes.Parametros `json:"-"`
	Piezas     []cortes.Pieza    `json:"-"`
}

// PlanesDeCorte vuelve a optimizar cada placa del presupuesto aprovechando los
// retazos dados (los del depósito de donde sale). El presupuesto se cobra con
// placas enteras; los retazos se aprovechan recién al cortar.
func PlanesDeCorte(mat Materiales, retazos map[int64][]cortes.Retazo) ([]cortes.PlanDeCorte, error) {
	var out []cortes.PlanDeCorte
	for _, pc := range mat.Placas {
		par := pc.Parametros
		par.Retazos = retazos[pc.ProductoID]
		res, err := cortes.Optimizar(par, pc.Piezas)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", pc.Nombre, err)
		}
		par.Retazos = nil
		out = append(out, cortes.PlanDeCorte{PlacaID: pc.ProductoID, Nombre: pc.Nombre, Parametros: par, Piezas: pc.Piezas, Resultado: res})
	}
	return out, nil
}

// MaterialCalculado es un canto (en metros) o un insumo.
type MaterialCalculado struct {
	ProductoID    int64   `json:"producto_id"`
	Nombre        string  `json:"nombre"`
	Unidad        string  `json:"unidad"`
	Cantidad      float64 `json:"cantidad"`
	CostoUnitario float64 `json:"costo_unitario"`
	Costo         float64 `json:"costo"`
}

// Materiales es todo lo que hay que usar o comprar para el presupuesto.
type Materiales struct {
	Placas  []PlacaCalculada    `json:"placas"`
	Cantos  []MaterialCalculado `json:"cantos"`
	Insumos []MaterialCalculado `json:"insumos"`
	Costo   float64             `json:"costo"`
	// CostoPorMueble es el costo de materiales de una unidad de cada mueble.
	// Las placas se cobran enteras y su costo se reparte según la superficie
	// de piezas de cada mueble.
	CostoPorMueble []float64 `json:"costo_por_mueble"`
	// PrecioPorMueble es el precio de una unidad: costo × Recargo, redondeado
	// a la centena para arriba.
	PrecioPorMueble []float64 `json:"precio_por_mueble"`
	// CantoPorMueble son los metros de canto de una unidad de cada mueble.
	CantoPorMueble []float64 `json:"canto_por_mueble"`
	// Cortes son los planes que quedan pendientes al confirmar (ver PlanesDeCorte).
	Cortes []cortes.PlanDeCorte `json:"-"`
}

// Recargo es por cuánto se multiplica el costo de materiales para el precio.
const Recargo = 2.7

// PrecioDe es el precio de venta para un costo de materiales.
func PrecioDe(costo float64) float64 {
	if costo <= 0 {
		return 0
	}
	return math.Ceil(redondear(costo*Recargo)/100) * 100
}

func redondear(x float64) float64 { return math.Round(x*100) / 100 }

// Calcular optimiza las piezas de todos los muebles juntas, una vez por
// cada placa, y suma cantos e insumos. Los muebles tienen que estar validados.
//
// retazos son los sobrantes disponibles de cada placa: se usan antes que las
// placas enteras y no se cobran (ya se pagaron con la placa de la que salieron).
func Calcular(ms []Mueble, productos map[int64]stock.Producto, kerf int, retazos map[int64][]cortes.Retazo) (Materiales, error) {
	out := Materiales{Placas: []PlacaCalculada{}, Cantos: []MaterialCalculado{}, Insumos: []MaterialCalculado{}, CostoPorMueble: make([]float64, len(ms)),
		PrecioPorMueble: make([]float64, len(ms)), CantoPorMueble: make([]float64, len(ms))}

	// Piezas por placa, con la cantidad multiplicada por la de muebles.
	porPlaca := map[int64][]cortes.Pieza{}
	areaPorPlaca := map[int64]float64{}
	areaMueble := make([]map[int64]float64, len(ms)) // por unidad de mueble
	cantoPorProducto := map[int64][]cortes.Pieza{}
	cantoMueble := make([]map[int64][]cortes.Pieza, len(ms))
	insumos := map[int64]float64{}
	for i, m := range ms {
		areaMueble[i] = map[int64]float64{}
		cantoMueble[i] = map[int64][]cortes.Pieza{}
		for j, p := range m.Piezas {
			nombre := p.Nombre
			if nombre == "" {
				nombre = fmt.Sprintf("Pieza %d", j+1)
			}
			if len(ms) > 1 {
				nombre = m.Nombre + " · " + nombre
			}
			cp := cortes.Pieza{
				ID: strconv.Itoa(i) + "-" + strconv.Itoa(j), Etiqueta: nombre,
				Largo: p.Largo, Ancho: p.Ancho, Cantidad: p.Cantidad * m.Cantidad,
				RespetaVeta: p.RespetaVeta, VetaVertical: p.VetaVertical, Cantos: p.Cantos,
			}
			porPlaca[p.PlacaID] = append(porPlaca[p.PlacaID], cp)
			a := float64(p.Largo*p.Ancho) * float64(p.Cantidad)
			areaPorPlaca[p.PlacaID] += a * float64(m.Cantidad)
			areaMueble[i][p.PlacaID] += a
			if p.CantoID != 0 {
				cantoPorProducto[p.CantoID] = append(cantoPorProducto[p.CantoID], cp)
				uno := cp
				uno.Cantidad = p.Cantidad
				cantoMueble[i][p.CantoID] = append(cantoMueble[i][p.CantoID], uno)
			}
		}
		for _, ins := range m.Insumos {
			insumos[ins.ProductoID] += ins.Cantidad * float64(m.Cantidad)
		}
	}

	costoPorArea := map[int64]float64{} // costo de placa por mm² de pieza
	for _, id := range claves(porPlaca) {
		pr := productos[id]
		par := cortes.Parametros{
			PlacaLargo: pr.LargoMM, PlacaAncho: pr.AnchoMM, TieneVeta: pr.TieneVeta,
			Kerf: kerf, Refilado: pr.RefiladoMM, SobranteMinLargo: cortes.SobranteMinLargo, SobranteMinAncho: cortes.SobranteMinAncho,
		}
		con := par
		con.Retazos = retazos[id]
		res, err := cortes.Optimizar(con, porPlaca[id])
		if err != nil {
			return out, fmt.Errorf("%s: %w", pr.Nombre, err)
		}
		if len(res.NoColocadas) > 0 {
			return out, ErrValidacion{"piezas", fmt.Sprintf("%s: hay piezas que no entran en la placa", pr.Nombre)}
		}
		pc := PlacaCalculada{
			ProductoID: id, Nombre: pr.Nombre, Cantidad: res.PlacasEnteras, Retazos: res.RetazosUsados, Aprovechamiento: res.Aprovechamiento,
			CostoUnitario: pr.Costo, Costo: redondear(float64(res.PlacasEnteras) * pr.Costo), Plan: res,
			Parametros: par, Piezas: porPlaca[id],
		}
		out.Placas = append(out.Placas, pc)
		out.Costo += pc.Costo
		if areaPorPlaca[id] > 0 {
			costoPorArea[id] = pc.Costo / areaPorPlaca[id]
		}
	}

	for _, id := range claves(cantoPorProducto) {
		pr := productos[id]
		metros := cortes.MetrosCanto(cantoPorProducto[id], cortes.MermaCantoMM)
		mc := MaterialCalculado{ProductoID: id, Nombre: pr.Nombre, Unidad: "metro", Cantidad: metros,
			CostoUnitario: pr.Costo, Costo: redondear(metros * pr.Costo)}
		out.Cantos = append(out.Cantos, mc)
		out.Costo += mc.Costo
	}
	for _, id := range claves(insumos) {
		pr := productos[id]
		mc := MaterialCalculado{ProductoID: id, Nombre: pr.Nombre, Unidad: stock.UnidadDe(pr.Tipo), Cantidad: insumos[id],
			CostoUnitario: pr.Costo, Costo: redondear(insumos[id] * pr.Costo)}
		out.Insumos = append(out.Insumos, mc)
		out.Costo += mc.Costo
	}
	out.Costo = redondear(out.Costo)

	for i, m := range ms {
		c := 0.0
		for id, a := range areaMueble[i] {
			c += a * costoPorArea[id]
		}
		for id, ps := range cantoMueble[i] {
			m := cortes.MetrosCanto(ps, cortes.MermaCantoMM)
			c += m * productos[id].Costo
			out.CantoPorMueble[i] += m
		}
		out.CantoPorMueble[i] = redondear(out.CantoPorMueble[i])
		for _, ins := range m.Insumos {
			c += ins.Cantidad * productos[ins.ProductoID].Costo
		}
		out.CostoPorMueble[i] = redondear(c)
		out.PrecioPorMueble[i] = PrecioDe(out.CostoPorMueble[i])
	}
	return out, nil
}

// ProductosUsados son los ids de todos los productos que nombran los muebles.
func ProductosUsados(ms []Mueble) []int64 {
	set := map[int64]bool{}
	for _, m := range ms {
		for _, p := range m.Piezas {
			set[p.PlacaID] = true
			if p.CantoID != 0 {
				set[p.CantoID] = true
			}
		}
		for _, ins := range m.Insumos {
			set[ins.ProductoID] = true
		}
	}
	return claves(set)
}

func claves[V any](m map[int64]V) []int64 {
	out := make([]int64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
