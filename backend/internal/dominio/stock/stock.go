// Package stock tiene las entidades y reglas de productos, depósitos y
// movimientos de stock. No depende de la base de datos ni de HTTP.
package stock

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNoEncontrado      = errors.New("no encontrado")
	ErrStockInsuficiente = errors.New("no hay stock suficiente")
	ErrMismoDeposito     = errors.New("el depósito de origen y el de destino son el mismo")
	ErrCantidad          = errors.New("la cantidad tiene que ser mayor a cero")
	ErrSkuEnUso          = errors.New("ya hay un producto con ese código")
	ErrCategoriaEnUso    = errors.New("ya hay una categoría con ese nombre")
)

// ErrValidacion es un error de datos cargados por el usuario.
type ErrValidacion struct{ Campo, Mensaje string }

func (e ErrValidacion) Error() string { return e.Mensaje }

type Tipo string

const (
	TipoPlaca   Tipo = "placa"
	TipoCanto   Tipo = "canto"
	TipoHerraje Tipo = "herraje"
	TipoOtro    Tipo = "otro"
)

// UnidadDe es la unidad en que se cuenta el stock de cada tipo.
func UnidadDe(t Tipo) string {
	switch t {
	case TipoPlaca:
		return "placa"
	case TipoCanto:
		return "metro"
	default:
		return "unidad"
	}
}

// Producto de la carpintería. Las medidas están en milímetros.
type Producto struct {
	ID           int64   `json:"id"`
	Tipo         Tipo    `json:"tipo"`
	Sku          string  `json:"sku"`
	Nombre       string  `json:"nombre"`
	Marca        string  `json:"marca"`
	Color        string  `json:"color"`
	Unidad       string  `json:"unidad"`
	Costo        float64 `json:"costo"`
	PrecioVenta  float64 `json:"precio_venta"`
	StockMinimo  float64 `json:"stock_minimo"`
	LargoMM      int     `json:"largo_mm,omitempty"`
	AnchoMM      int     `json:"ancho_mm,omitempty"`
	EspesorMM    float64 `json:"espesor_mm,omitempty"`
	TieneVeta    bool    `json:"tiene_veta"`
	RefiladoMM   int     `json:"refilado_mm"`
	CantoAnchoMM int     `json:"canto_ancho_mm,omitempty"`
	CantoEspMM   float64 `json:"canto_espesor_mm,omitempty"`
	Activo       bool    `json:"activo"`
	// CategoriaID es opcional. Categoria es el nombre, solo para mostrar.
	CategoriaID *int64 `json:"categoria_id"`
	Categoria   string `json:"categoria,omitempty"`
}

// Normalizar limpia los datos y completa la unidad según el tipo.
func (p *Producto) Normalizar() {
	p.Nombre = strings.TrimSpace(p.Nombre)
	p.Sku = strings.TrimSpace(p.Sku)
	p.Marca = strings.TrimSpace(p.Marca)
	p.Color = strings.TrimSpace(p.Color)
	p.Unidad = UnidadDe(p.Tipo)
	p.Categoria = ""
	if p.CategoriaID != nil && *p.CategoriaID <= 0 {
		p.CategoriaID = nil
	}
	if p.Tipo != TipoPlaca {
		p.LargoMM, p.AnchoMM, p.EspesorMM, p.TieneVeta, p.RefiladoMM = 0, 0, 0, false, 0
	}
	if p.Tipo != TipoCanto {
		p.CantoAnchoMM, p.CantoEspMM = 0, 0
	}
}

// Validar controla los datos de un producto.
func (p Producto) Validar() error {
	switch p.Tipo {
	case TipoPlaca, TipoCanto, TipoHerraje, TipoOtro:
	default:
		return ErrValidacion{"tipo", "el tipo de producto no es válido"}
	}
	if p.Nombre == "" {
		return ErrValidacion{"nombre", "falta el nombre"}
	}
	if p.Costo < 0 || p.PrecioVenta < 0 || p.StockMinimo < 0 {
		return ErrValidacion{"precio", "los precios y el stock mínimo no pueden ser negativos"}
	}
	if p.Tipo == TipoPlaca {
		if p.LargoMM <= 0 || p.AnchoMM <= 0 || p.EspesorMM <= 0 {
			return ErrValidacion{"medidas", "la placa necesita largo, ancho y espesor"}
		}
		if p.RefiladoMM < 0 || 2*p.RefiladoMM >= min(p.LargoMM, p.AnchoMM) {
			return ErrValidacion{"refilado_mm", "el refilado no es válido para esa placa"}
		}
	}
	return nil
}

// Categoria agrupa productos (ej: placas, bisagras, guías). La arma cada
// carpintería a su gusto; no cambia cómo se cuenta el stock.
type Categoria struct {
	ID          int64  `json:"id"`
	Nombre      string `json:"nombre"`
	Descripcion string `json:"descripcion"`
	// Productos es cuántos productos activos tiene; solo para mostrar.
	Productos int `json:"productos"`
}

// Normalizar limpia los textos de la categoría.
func (c *Categoria) Normalizar() {
	c.Nombre = strings.Join(strings.Fields(c.Nombre), " ")
	c.Descripcion = strings.TrimSpace(c.Descripcion)
}

// Validar controla los datos de una categoría.
func (c Categoria) Validar() error {
	if c.Nombre == "" {
		return ErrValidacion{"nombre", "falta el nombre de la categoría"}
	}
	if len([]rune(c.Nombre)) > 60 {
		return ErrValidacion{"nombre", "el nombre de la categoría es muy largo (máximo 60 letras)"}
	}
	if len([]rune(c.Descripcion)) > 300 {
		return ErrValidacion{"descripcion", "la descripción es muy larga (máximo 300 letras)"}
	}
	return nil
}

// Deposito de una sucursal.
type Deposito struct {
	ID         int64  `json:"id"`
	Nombre     string `json:"nombre"`
	SucursalID int64  `json:"sucursal_id"`
	Sucursal   string `json:"sucursal"`
}

// TipoMovimiento coincide con el enum tipo_movimiento de la base.
type TipoMovimiento string

const (
	MovCompra          TipoMovimiento = "compra"
	MovVenta           TipoMovimiento = "venta"
	MovAjuste          TipoMovimiento = "ajuste"
	MovTraspasoEntrada TipoMovimiento = "traspaso_entrada"
	MovTraspasoSalida  TipoMovimiento = "traspaso_salida"
	MovDevolucion      TipoMovimiento = "devolucion"
)

// Movimiento de stock. Cantidad positiva entra, negativa sale.
type Movimiento struct {
	ProductoID    int64          `json:"producto_id"`
	DepositoID    int64          `json:"deposito_id"`
	Tipo          TipoMovimiento `json:"tipo"`
	Cantidad      float64        `json:"cantidad"`
	CostoUnitario *float64       `json:"costo_unitario,omitempty"`
	Nota          string         `json:"nota,omitempty"`
}

// Operacion es un movimiento junto con la regla que calcula el saldo nuevo a
// partir del saldo actual. El repositorio la aplica con el saldo bloqueado,
// así dos usuarios a la vez no pisan el stock.
type Operacion struct {
	Movimiento
	Nuevo func(saldo float64) (float64, error) `json:"-"`
}

// sumar devuelve la regla de sumar (o restar) una cantidad sin quedar en negativo.
func sumar(cantidad float64) func(float64) (float64, error) {
	return func(saldo float64) (float64, error) {
		nuevo := saldo + cantidad
		if nuevo < -1e-9 {
			return saldo, fmt.Errorf("%w: hay %s y se quieren sacar %s", ErrStockInsuficiente, formato(saldo), formato(-cantidad))
		}
		return nuevo, nil
	}
}

// Ingreso es una entrada de mercadería (compra).
func Ingreso(productoID, depositoID int64, cantidad float64, costo *float64, nota string) (Operacion, error) {
	if cantidad <= 0 {
		return Operacion{}, ErrCantidad
	}
	if costo != nil && *costo < 0 {
		return Operacion{}, ErrValidacion{"costo", "el costo no puede ser negativo"}
	}
	m := Movimiento{productoID, depositoID, MovCompra, cantidad, costo, strings.TrimSpace(nota)}
	return Operacion{m, sumar(cantidad)}, nil
}

// Ajuste lleva el stock al valor contado. El motivo es obligatorio para que
// quede registrado por qué se tocó. La cantidad del movimiento es la diferencia.
func Ajuste(productoID, depositoID int64, contado float64, motivo string) (Operacion, error) {
	if contado < 0 {
		return Operacion{}, ErrValidacion{"cantidad", "el stock contado no puede ser negativo"}
	}
	motivo = strings.TrimSpace(motivo)
	if motivo == "" {
		return Operacion{}, ErrValidacion{"nota", "contá el motivo del ajuste"}
	}
	m := Movimiento{ProductoID: productoID, DepositoID: depositoID, Tipo: MovAjuste, Nota: motivo}
	return Operacion{m, func(saldo float64) (float64, error) {
		if contado == saldo {
			return saldo, ErrValidacion{"cantidad", "el stock contado es igual al actual"}
		}
		return contado, nil
	}}, nil
}

// Traspaso pasa mercadería de un depósito a otro: una salida y una entrada.
func Traspaso(productoID, origen, destino int64, cantidad float64, nota string) ([]Operacion, error) {
	if cantidad <= 0 {
		return nil, ErrCantidad
	}
	if origen == destino {
		return nil, ErrMismoDeposito
	}
	nota = strings.TrimSpace(nota)
	return []Operacion{
		{Movimiento{productoID, origen, MovTraspasoSalida, -cantidad, nil, nota}, sumar(-cantidad)},
		{Movimiento{productoID, destino, MovTraspasoEntrada, cantidad, nil, nota}, sumar(cantidad)},
	}, nil
}

// Venta saca mercadería de un depósito para un pedido confirmado.
func Venta(productoID, depositoID int64, cantidad float64, nota string) (Operacion, error) {
	if cantidad <= 0 {
		return Operacion{}, ErrCantidad
	}
	m := Movimiento{productoID, depositoID, MovVenta, -cantidad, nil, strings.TrimSpace(nota)}
	return Operacion{m, sumar(-cantidad)}, nil
}

// Devolucion vuelve a entrar mercadería de un pedido cancelado.
func Devolucion(productoID, depositoID int64, cantidad float64, nota string) (Operacion, error) {
	if cantidad <= 0 {
		return Operacion{}, ErrCantidad
	}
	m := Movimiento{productoID, depositoID, MovDevolucion, cantidad, nil, strings.TrimSpace(nota)}
	return Operacion{m, sumar(cantidad)}, nil
}

// EstaBajo dice si el stock total está por debajo del mínimo.
func EstaBajo(total, minimo float64) bool { return minimo > 0 && total < minimo }

func formato(f float64) string {
	s := fmt.Sprintf("%.3f", f)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	return strings.Replace(s, ".", ",", 1)
}
