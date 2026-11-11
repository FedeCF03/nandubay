// Package ventas tiene las reglas de presupuestos, ventas, pagos y retiros.
// Una venta nace como presupuesto; al confirmarla se elige de qué depósito
// sale la mercadería y se descuenta el stock. El cliente la retira, de una vez
// o por partes, en la sucursal de ese depósito mostrando la boleta.
package ventas

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/FedeCF03/nandubay/backend/internal/modelos/muebles"
	"github.com/FedeCF03/nandubay/backend/internal/modelos/stock"
)

var (
	ErrNoEncontrado   = errors.New("no encontrado")
	ErrEstado         = errors.New("la venta no está en un estado que permita esto")
	ErrSinLineas      = errors.New("la venta no tiene nada cargado")
	ErrSaldoPendiente = errors.New("la venta tiene saldo pendiente: para retirar tiene que estar pagada")
	ErrConPagos       = errors.New("la venta tiene pagos: registrá la devolución antes de cancelarla")
	ErrConRetiros     = errors.New("el cliente ya retiró mercadería de esta venta")
	ErrOtraSucursal   = errors.New("esta venta se retira en otra sucursal")
	ErrPagoExcedido   = errors.New("el pago supera el saldo de la venta")
	ErrNadaQueRetirar = errors.New("no hay nada para retirar")
	ErrMuebleNoListo  = errors.New("los muebles se retiran cuando el pedido está listo")
)

// ErrValidacion es un error en los datos cargados.
type ErrValidacion struct{ Campo, Mensaje string }

func (e ErrValidacion) Error() string { return e.Mensaje }

type Estado string

const (
	Presupuesto Estado = "presupuesto"
	Confirmado  Estado = "confirmado"
	EnCorte     Estado = "en_corte"
	EnArmado    Estado = "en_armado"
	Listo       Estado = "listo"
	Entregado   Estado = "entregado"
	Cancelado   Estado = "cancelado"
)

// EstadosProduccion son los que se cambian a mano mientras se fabrica.
var EstadosProduccion = []Estado{Confirmado, EnCorte, EnArmado, Listo}

func esProduccion(e Estado) bool {
	for _, x := range EstadosProduccion {
		if x == e {
			return true
		}
	}
	return false
}

// Cliente de la carpintería.
type Cliente struct {
	ID        int64  `json:"id"`
	Nombre    string `json:"nombre"`
	Documento string `json:"documento"`
	Telefono  string `json:"telefono"`
	Email     string `json:"email"`
	Direccion string `json:"direccion"`
	Notas     string `json:"notas"`
}

// Normalizar limpia espacios y valida el cliente.
func (c *Cliente) Normalizar() error {
	for _, s := range []*string{&c.Nombre, &c.Documento, &c.Telefono, &c.Email, &c.Direccion, &c.Notas} {
		*s = strings.TrimSpace(*s)
	}
	c.Email = strings.ToLower(c.Email)
	if c.Nombre == "" {
		return ErrValidacion{"nombre", "poné el nombre del cliente"}
	}
	if len(c.Nombre) > 200 {
		return ErrValidacion{"nombre", "el nombre es demasiado largo"}
	}
	if c.Email != "" && !strings.Contains(c.Email, "@") {
		return ErrValidacion{"email", "el email no es válido"}
	}
	return nil
}

// TipoVenta separa las ventas de mostrador (productos del stock) de los
// presupuestos de muebles, que van en otra sección.
type TipoVenta string

const (
	Mostrador TipoVenta = "mostrador"
	Muebles   TipoVenta = "muebles"
)

// LineasDeMuebles arma una línea por mueble del presupuesto. costos es el
// costo de materiales de cada unidad (para el margen).
func LineasDeMuebles(ms []muebles.Mueble, costos []float64) []Linea {
	out := make([]Linea, len(ms))
	for i, m := range ms {
		out[i] = Linea{
			Tipo: LineaMueble, Descripcion: m.Nombre, Unidad: "unidad",
			AnchoMM: m.AnchoMM, AltoMM: m.AltoMM, ProfundidadMM: m.ProfundidadMM,
			Cantidad: float64(m.Cantidad), PrecioUnitario: Redondear(m.Precio),
		}
		if i < len(costos) {
			out[i].CostoUnitario = costos[i]
		}
	}
	return out
}

// ControlarLineas dice si las líneas corresponden al tipo de venta: en el
// mostrador solo se venden productos del stock.
func ControlarLineas(t TipoVenta, ls []Linea) error {
	for _, l := range ls {
		if t == Mostrador && l.Tipo != LineaProducto {
			return ErrValidacion{"lineas", "en el mostrador solo se venden productos del stock: los muebles van en presupuestos de muebles"}
		}
		if t == Muebles && l.Tipo != LineaMueble {
			return ErrValidacion{"lineas", "un presupuesto de muebles lleva muebles: los productos sueltos se venden en el mostrador"}
		}
	}
	return nil
}

type TipoLinea string

const (
	LineaProducto TipoLinea = "producto" // sale del stock
	LineaMueble   TipoLinea = "mueble"   // se fabrica
)

// Linea es un renglón de la venta: un producto del stock o un mueble.
type Linea struct {
	ID             int64     `json:"id"`
	Tipo           TipoLinea `json:"tipo"`
	ProductoID     int64     `json:"producto_id,omitempty"`
	Descripcion    string    `json:"descripcion"`
	Unidad         string    `json:"unidad"`
	AnchoMM        int       `json:"ancho_mm,omitempty"`
	AltoMM         int       `json:"alto_mm,omitempty"`
	ProfundidadMM  int       `json:"profundidad_mm,omitempty"`
	Cantidad       float64   `json:"cantidad"`
	PrecioUnitario float64   `json:"precio_unitario"`
	CostoUnitario  float64   `json:"-"`
	Retirado       float64   `json:"retirado"`
}

func (l Linea) Subtotal() float64  { return Redondear(l.Cantidad * l.PrecioUnitario) }
func (l Linea) Pendiente() float64 { return math.Max(0, l.Cantidad-l.Retirado) }

// LineaNueva es lo que carga el vendedor. Para un producto, si no pone precio
// se usa el de lista.
type LineaNueva struct {
	Tipo           TipoLinea `json:"tipo"`
	ProductoID     int64     `json:"producto_id"`
	Descripcion    string    `json:"descripcion"`
	AnchoMM        int       `json:"ancho_mm"`
	AltoMM         int       `json:"alto_mm"`
	ProfundidadMM  int       `json:"profundidad_mm"`
	Cantidad       float64   `json:"cantidad"`
	PrecioUnitario *float64  `json:"precio_unitario"`
}

// ArmarLineas valida lo cargado y completa precio, costo y unidad desde los
// productos. productos tiene que traer todos los productos nombrados.
func ArmarLineas(nuevas []LineaNueva, productos map[int64]stock.Producto) ([]Linea, error) {
	if len(nuevas) == 0 {
		return nil, ErrSinLineas
	}
	if len(nuevas) > 300 {
		return nil, ErrValidacion{"lineas", "demasiados renglones"}
	}
	out := make([]Linea, 0, len(nuevas))
	for i, n := range nuevas {
		campo := func(c string) string { return fmt.Sprintf("lineas.%d.%s", i, c) }
		if n.Cantidad <= 0 || n.Cantidad > 1e7 {
			return nil, ErrValidacion{campo("cantidad"), fmt.Sprintf("renglón %d: la cantidad tiene que ser mayor a cero", i+1)}
		}
		if n.PrecioUnitario != nil && (*n.PrecioUnitario < 0 || *n.PrecioUnitario > 1e10) {
			return nil, ErrValidacion{campo("precio_unitario"), fmt.Sprintf("renglón %d: el precio no es válido", i+1)}
		}
		l := Linea{Tipo: n.Tipo, Cantidad: n.Cantidad, Descripcion: strings.TrimSpace(n.Descripcion)}
		switch n.Tipo {
		case LineaProducto:
			p, ok := productos[n.ProductoID]
			if !ok {
				return nil, ErrValidacion{campo("producto_id"), fmt.Sprintf("renglón %d: el producto no existe", i+1)}
			}
			l.ProductoID = p.ID
			if l.Descripcion == "" {
				l.Descripcion = p.Nombre
			}
			l.Unidad = stock.UnidadDe(p.Tipo)
			l.PrecioUnitario = p.PrecioVenta
			l.CostoUnitario = p.Costo
			if l.Unidad != "metro" && l.Cantidad != math.Trunc(l.Cantidad) {
				return nil, ErrValidacion{campo("cantidad"), fmt.Sprintf("renglón %d: %s se vende por unidad entera", i+1, p.Nombre)}
			}
		case LineaMueble:
			if l.Descripcion == "" {
				return nil, ErrValidacion{campo("descripcion"), fmt.Sprintf("renglón %d: describí el mueble", i+1)}
			}
			if l.Cantidad != math.Trunc(l.Cantidad) {
				return nil, ErrValidacion{campo("cantidad"), fmt.Sprintf("renglón %d: la cantidad de muebles es entera", i+1)}
			}
			if n.AnchoMM < 0 || n.AltoMM < 0 || n.ProfundidadMM < 0 || n.AnchoMM > 20000 || n.AltoMM > 20000 || n.ProfundidadMM > 20000 {
				return nil, ErrValidacion{campo("ancho_mm"), fmt.Sprintf("renglón %d: las medidas no son válidas", i+1)}
			}
			l.Unidad = "unidad"
			l.AnchoMM, l.AltoMM, l.ProfundidadMM = n.AnchoMM, n.AltoMM, n.ProfundidadMM
			if n.PrecioUnitario == nil {
				return nil, ErrValidacion{campo("precio_unitario"), fmt.Sprintf("renglón %d: poné el precio del mueble", i+1)}
			}
		default:
			return nil, ErrValidacion{campo("tipo"), fmt.Sprintf("renglón %d: tipo desconocido", i+1)}
		}
		if len(l.Descripcion) > 300 {
			return nil, ErrValidacion{campo("descripcion"), fmt.Sprintf("renglón %d: la descripción es demasiado larga", i+1)}
		}
		if n.PrecioUnitario != nil {
			l.PrecioUnitario = Redondear(*n.PrecioUnitario)
		}
		out = append(out, l)
	}
	return out, nil
}

type TipoPago string

const (
	PagoSena       TipoPago = "sena"
	PagoPago       TipoPago = "pago"
	PagoDevolucion TipoPago = "devolucion"
)

type Pago struct {
	ID      int64     `json:"id"`
	Tipo    TipoPago  `json:"tipo"`
	Monto   float64   `json:"monto"`
	Medio   string    `json:"medio"`
	Nota    string    `json:"nota"`
	Usuario string    `json:"usuario"`
	Fecha   time.Time `json:"fecha"`
}

// RetiroItem es cuánto se lleva de una línea.
type RetiroItem struct {
	LineaID     int64     `json:"linea_id"`
	Tipo        TipoLinea `json:"tipo"`
	Descripcion string    `json:"descripcion,omitempty"`
	Cantidad    float64   `json:"cantidad"`
}

type Retiro struct {
	ID       int64        `json:"id"`
	Sucursal string       `json:"sucursal"`
	Usuario  string       `json:"usuario"`
	Nota     string       `json:"nota"`
	Fecha    time.Time    `json:"fecha"`
	Items    []RetiroItem `json:"items"`
}

// Lugar es una sucursal con los datos que van en la boleta.
type Lugar struct {
	ID        int64  `json:"id"`
	Nombre    string `json:"nombre"`
	Direccion string `json:"direccion"`
	Telefono  string `json:"telefono"`
}

type CambioEstado struct {
	Estado  Estado    `json:"estado"`
	Usuario string    `json:"usuario"`
	Fecha   time.Time `json:"fecha"`
}

// Pedido es una venta, desde que es presupuesto hasta que se entrega.
type Pedido struct {
	ID           int64          `json:"id"`
	Numero       int            `json:"numero"`
	Tipo         TipoVenta      `json:"tipo"`
	Estado       Estado         `json:"estado"`
	Sucursal     Lugar          `json:"sucursal"` // donde se vendió
	DepositoID   *int64         `json:"deposito_id"`
	Deposito     string         `json:"deposito"`
	Retira       *Lugar         `json:"retira"` // sucursal del depósito: donde se retira
	Cliente      Cliente        `json:"cliente"`
	Vendedor     string         `json:"vendedor"`
	Lineas       []Linea        `json:"lineas"`
	Pagos        []Pago         `json:"pagos"`
	Retiros      []Retiro       `json:"retiros"`
	Historial    []CambioEstado `json:"historial"`
	Notas        string         `json:"notas"`
	FechaEntrega *string        `json:"fecha_entrega"`
	CreadoEn     time.Time      `json:"creado_en"`
	ConfirmadoEn *time.Time     `json:"confirmado_en"`
	// Consumido es lo que salió del stock al confirmar, por producto (de los
	// movimientos guardados). Al cancelar se devuelve exactamente eso.
	Consumido []Consumo `json:"-"`
}

// Consumo es una cantidad de un producto del stock que usa la venta: los
// renglones de productos o los materiales calculados de los muebles.
type Consumo struct {
	ProductoID int64   `json:"producto_id"`
	Cantidad   float64 `json:"cantidad"`
}

func Redondear(x float64) float64 { return math.Round(x*100) / 100 }

func (p Pedido) Total() float64 {
	t := 0.0
	for _, l := range p.Lineas {
		t += l.Subtotal()
	}
	return Redondear(t)
}

func (p Pedido) Costo() float64 {
	t := 0.0
	for _, l := range p.Lineas {
		t += l.Cantidad * l.CostoUnitario
	}
	return Redondear(t)
}

// Pagado es lo cobrado menos lo devuelto.
func (p Pedido) Pagado() float64 {
	t := 0.0
	for _, x := range p.Pagos {
		if x.Tipo == PagoDevolucion {
			t -= x.Monto
		} else {
			t += x.Monto
		}
	}
	return Redondear(t)
}

func (p Pedido) Saldo() float64 { return Redondear(p.Total() - p.Pagado()) }

// EstaPagada dice si la boleta puede decir "PAGADA".
func (p Pedido) EstaPagada() bool {
	return p.Estado != Presupuesto && p.Estado != Cancelado && p.Saldo() <= 0
}

// TodoRetirado dice si ya no queda nada por entregar.
func (p Pedido) TodoRetirado() bool {
	for _, l := range p.Lineas {
		if l.Pendiente() > 1e-9 {
			return false
		}
	}
	return true
}

func (p Pedido) confirmada() bool { return esProduccion(p.Estado) }

// Cambio es lo que hay que guardar después de una operación sobre la venta.
// El repositorio lo aplica en la misma transacción en que leyó la venta.
type Cambio struct {
	Estado      *Estado
	DepositoID  *int64
	Lineas      []Linea // si no es nil reemplaza todas las líneas
	Datos       *DatosVenta
	Operaciones []stock.Operacion
	Pago        *Pago
	Retiro      *RetiroNuevo
	// Despiece acompaña a Lineas en un presupuesto de muebles: piezas e
	// insumos de cada mueble, en el mismo orden que las líneas.
	Despiece []muebles.Mueble
	// Materiales es el plan con que se confirma un presupuesto de muebles:
	// se guarda y marca los retazos usados y los sobrantes nuevos.
	Materiales *muebles.Materiales
}

// DatosVenta son los datos generales que se editan mientras es presupuesto.
type DatosVenta struct {
	ClienteID    int64   `json:"cliente_id"`
	Notas        string  `json:"notas"`
	FechaEntrega *string `json:"fecha_entrega"`
}

// RetiroNuevo es un retiro a registrar.
type RetiroNuevo struct {
	SucursalID int64
	Nota       string
	Items      []RetiroItem
}

func ptr[T any](v T) *T { return &v }

// Editar reemplaza las líneas y datos de un presupuesto.
func (p Pedido) Editar(d DatosVenta, lineas []Linea) (Cambio, error) {
	if p.Estado != Presupuesto {
		return Cambio{}, fmt.Errorf("%w: solo se puede editar un presupuesto", ErrEstado)
	}
	if len(lineas) == 0 {
		return Cambio{}, ErrSinLineas
	}
	if err := ControlarLineas(p.Tipo, lineas); err != nil {
		return Cambio{}, err
	}
	return Cambio{Lineas: lineas, Datos: &d}, nil
}

// Confirmar pasa el presupuesto a venta y descuenta el stock del depósito
// elegido, que puede ser de otra sucursal.
// Confirmar pasa el presupuesto a venta y descuenta del depósito los
// productos de los renglones más los materiales de los muebles (placas,
// canto e insumos calculados del despiece).
func (p Pedido) Confirmar(depositoID int64, materiales []Consumo) (Cambio, error) {
	if p.Estado != Presupuesto {
		return Cambio{}, fmt.Errorf("%w: ya está confirmada", ErrEstado)
	}
	if len(p.Lineas) == 0 {
		return Cambio{}, ErrSinLineas
	}
	c := Cambio{Estado: ptr(Confirmado), DepositoID: &depositoID}
	for _, x := range sumarConsumos(append(p.consumosDeLineas(), materiales...)) {
		op, err := stock.Venta(x.ProductoID, depositoID, x.Cantidad, fmt.Sprintf("Venta N° %d", p.Numero))
		if err != nil {
			return Cambio{}, err
		}
		c.Operaciones = append(c.Operaciones, op)
	}
	return c, nil
}

// RegistrarPago suma una seña o pago, o resta una devolución.
func (p Pedido) RegistrarPago(pago Pago) (Cambio, error) {
	if p.Estado == Cancelado {
		return Cambio{}, fmt.Errorf("%w: está cancelada", ErrEstado)
	}
	pago.Medio = strings.TrimSpace(pago.Medio)
	pago.Nota = strings.TrimSpace(pago.Nota)
	pago.Monto = Redondear(pago.Monto)
	if pago.Monto <= 0 {
		return Cambio{}, ErrValidacion{"monto", "el monto tiene que ser mayor a cero"}
	}
	if pago.Medio == "" {
		return Cambio{}, ErrValidacion{"medio", "elegí cómo pagó"}
	}
	switch pago.Tipo {
	case PagoSena, PagoPago:
		if pago.Monto > p.Saldo()+0.001 {
			return Cambio{}, fmt.Errorf("%w: el saldo es $%.2f", ErrPagoExcedido, p.Saldo())
		}
	case PagoDevolucion:
		if pago.Monto > p.Pagado()+0.001 {
			return Cambio{}, ErrValidacion{"monto", "no se puede devolver más de lo que pagó"}
		}
	default:
		return Cambio{}, ErrValidacion{"tipo", "tipo de pago desconocido"}
	}
	return Cambio{Pago: &pago}, nil
}

// CambiarEstado mueve la venta entre los estados de fabricación. Entregada se
// pone sola cuando el cliente retira todo.
func (p Pedido) CambiarEstado(nuevo Estado) (Cambio, error) {
	if !p.confirmada() || !esProduccion(nuevo) {
		return Cambio{}, ErrEstado
	}
	if nuevo == p.Estado {
		return Cambio{}, fmt.Errorf("%w: ya está en ese estado", ErrEstado)
	}
	return Cambio{Estado: &nuevo}, nil
}

// Retirar registra que el cliente se lleva parte o todo. Solo se puede en la
// sucursal del depósito de donde salió el stock, y con la venta paga.
func (p Pedido) Retirar(sucursalID int64, items []RetiroItem, nota string) (Cambio, error) {
	if !p.confirmada() {
		return Cambio{}, fmt.Errorf("%w: solo se retira una venta confirmada y no entregada", ErrEstado)
	}
	if p.Retira == nil || p.Retira.ID != sucursalID {
		donde := ""
		if p.Retira != nil {
			donde = " (" + p.Retira.Nombre + ")"
		}
		return Cambio{}, fmt.Errorf("%w%s", ErrOtraSucursal, donde)
	}
	if p.Saldo() > 0 {
		return Cambio{}, fmt.Errorf("%w (faltan $%.2f)", ErrSaldoPendiente, p.Saldo())
	}
	// Los productos y los muebles se guardan aparte: el id solo es único
	// dentro de cada tipo.
	type clave struct {
		tipo TipoLinea
		id   int64
	}
	lineas := map[clave]*Linea{}
	nuevas := make([]Linea, len(p.Lineas))
	copy(nuevas, p.Lineas)
	for i := range nuevas {
		lineas[clave{nuevas[i].Tipo, nuevas[i].ID}] = &nuevas[i]
	}
	var validos []RetiroItem
	for _, it := range items {
		if it.Cantidad == 0 {
			continue
		}
		l, ok := lineas[clave{it.Tipo, it.LineaID}]
		if !ok {
			return Cambio{}, ErrValidacion{"items", "un renglón no es de esta venta"}
		}
		if it.Cantidad < 0 {
			return Cambio{}, ErrValidacion{"items", "la cantidad no puede ser negativa"}
		}
		if it.Cantidad > l.Pendiente()+1e-9 {
			return Cambio{}, ErrValidacion{"items", fmt.Sprintf("de %s quedan %s por retirar", l.Descripcion, numero(l.Pendiente()))}
		}
		if l.Tipo == LineaMueble && p.Estado != Listo {
			return Cambio{}, ErrMuebleNoListo
		}
		if l.Unidad != "metro" && it.Cantidad != math.Trunc(it.Cantidad) {
			return Cambio{}, ErrValidacion{"items", fmt.Sprintf("%s se retira por unidad entera", l.Descripcion)}
		}
		l.Retirado += it.Cantidad
		validos = append(validos, RetiroItem{LineaID: l.ID, Tipo: l.Tipo, Descripcion: l.Descripcion, Cantidad: it.Cantidad})
	}
	if len(validos) == 0 {
		return Cambio{}, ErrNadaQueRetirar
	}
	c := Cambio{Retiro: &RetiroNuevo{SucursalID: sucursalID, Nota: strings.TrimSpace(nota), Items: validos}}
	if (Pedido{Lineas: nuevas}).TodoRetirado() {
		c.Estado = ptr(Entregado)
	}
	return c, nil
}

// Cancelar anula la venta. Si ya estaba confirmada, el stock vuelve al depósito.
func (p Pedido) Cancelar() (Cambio, error) {
	if p.Estado == Cancelado || p.Estado == Entregado {
		return Cambio{}, ErrEstado
	}
	if len(p.Retiros) > 0 {
		return Cambio{}, ErrConRetiros
	}
	if p.Pagado() > 0 {
		return Cambio{}, ErrConPagos
	}
	c := Cambio{Estado: ptr(Cancelado)}
	if p.confirmada() && p.DepositoID != nil {
		// Se devuelve lo que salió del stock; si no está registrado (ventas
		// viejas), lo que dicen los renglones.
		devolver := p.Consumido
		if len(devolver) == 0 {
			devolver = sumarConsumos(p.consumosDeLineas())
		}
		for _, x := range devolver {
			op, err := stock.Devolucion(x.ProductoID, *p.DepositoID, x.Cantidad, fmt.Sprintf("Venta N° %d cancelada", p.Numero))
			if err != nil {
				return Cambio{}, err
			}
			c.Operaciones = append(c.Operaciones, op)
		}
	}
	return c, nil
}

// consumosDeLineas son los renglones de productos del stock.
func (p Pedido) consumosDeLineas() []Consumo {
	var out []Consumo
	for _, l := range p.Lineas {
		if l.Tipo == LineaProducto {
			out = append(out, Consumo{l.ProductoID, l.Cantidad})
		}
	}
	return out
}

// sumarConsumos junta las cantidades de un mismo producto, en el orden en que
// aparecen, para mover el stock una sola vez por producto.
func sumarConsumos(cs []Consumo) []Consumo {
	var out []Consumo
	pos := map[int64]int{}
	for _, x := range cs {
		if x.Cantidad <= 0 {
			continue
		}
		if i, ok := pos[x.ProductoID]; ok {
			out[i].Cantidad += x.Cantidad
			continue
		}
		pos[x.ProductoID] = len(out)
		out = append(out, x)
	}
	return out
}

func numero(f float64) string {
	s := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", f), "0"), ".")
	return strings.Replace(s, ".", ",", 1)
}
