package ventas

import (
	"errors"
	"time"

	"github.com/FedeCF03/nandubay/backend/internal/model/muebles"
)

// Límites de las fotos de los muebles. El navegador las achica antes de
// mandarlas, así que una foto normal pesa unos cientos de KB.
const (
	MaxFotosPorMueble = 12
	MaxBytesFoto      = 4 << 20
)

var (
	ErrDemasiadasFotos = errors.New("el mueble ya tiene el máximo de fotos")
	ErrFotoInvalida    = errors.New("la foto tiene que ser JPG, PNG o WebP de hasta 4 MB")
)

// FiltroVentas para listar
type FiltroVentas struct {
	Tipo   TipoVenta // vacío = todas
	Estado string    // vacío = todas; "a_retirar" = confirmadas con algo pendiente
	Buscar string    // número de venta o nombre del cliente
	Limite int
}

// ResumenVenta es un renglón del listado de
type ResumenVenta struct {
	ID       int64     `json:"id"`
	Numero   int       `json:"numero"`
	Estado   Estado    `json:"estado"`
	Cliente  string    `json:"cliente"`
	Sucursal string    `json:"sucursal"`
	Retira   string    `json:"retira"`
	Total    float64   `json:"total"`
	Saldo    float64   `json:"saldo"`
	Fecha    time.Time `json:"fecha"`
}

// VentaNueva es un presupuesto a guardar.
type VentaNueva struct {
	Tipo       TipoVenta
	SucursalID int64
	VendedorID int64
	Datos      DatosVenta
	Lineas     []Linea
	Despiece   []muebles.Mueble // en un presupuesto de muebles, uno por línea
}

// FotoMueble es una foto de un mueble del presupuesto (sin la imagen).
type FotoMueble struct {
	ID          int64     `json:"id"`
	MuebleClave string    `json:"mueble_clave"`
	CreadoEn    time.Time `json:"creado_en"`
}

// FotoNueva es una foto a guardar, ya controlada.
type FotoNueva struct {
	MuebleClave string
	Tipo        string
	Datos       []byte
}

// FiltroVendidos limita el resumen de productos vendidos. Cuenta las ventas
// confirmadas entre Desde (incluido) y Hasta (excluido), por fecha de confirmación.
type FiltroVendidos struct {
	Desde, Hasta time.Time
	SucursalID   int64 // 0 = todas
}

// ProductoVendido es cuánto se vendió de un producto en el período.
type ProductoVendido struct {
	ProductoID int64   `json:"producto_id"`
	Nombre     string  `json:"nombre"`
	Tipo       string  `json:"tipo"`
	Unidad     string  `json:"unidad"`
	Cantidad   float64 `json:"cantidad"`
	Importe    float64 `json:"importe"`
	Costo      float64 `json:"costo"`
	Ventas     int     `json:"ventas"`
}
