package stock

import "time"

// FiltroProductos para listar productos.
type FiltroProductos struct {
	Tipo string
	// CategoriaID filtra por categoría; -1 son los productos sin categoría.
	CategoriaID int64
	Buscar      string
	Inactivos   bool
}

// ProductoConStock es un producto con su stock en cada depósito.
type ProductoConStock struct {
	Producto
	PorDeposito map[int64]float64 `json:"por_deposito"`
	Total       float64           `json:"total"`
	Bajo        bool              `json:"bajo"`
}

// MovimientoRegistrado es un movimiento ya guardado, para el historial.
type MovimientoRegistrado struct {
	ID int64 `json:"id"`
	Movimiento
	Deposito string    `json:"deposito"`
	Usuario  string    `json:"usuario"`
	Fecha    time.Time `json:"fecha"`
}

// FiltroRetazos para listar retazos disponibles.
type FiltroRetazos struct {
	PlacaID    int64 // 0 = todas
	DepositoID int64 // 0 = todos
}

// RetazoStock es un sobrante de placa guardado en un depósito.
type RetazoStock struct {
	ID         int64  `json:"id"`
	PlacaID    int64  `json:"placa_id"`
	Placa      string `json:"placa"`
	DepositoID int64  `json:"deposito_id"`
	Deposito   string `json:"deposito"`
	Largo      int    `json:"largo"`
	Ancho      int    `json:"ancho"`
	Nota       string `json:"nota"`
	// Origen dice de dónde salió: "Venta N° 12" o vacío si se cargó a mano.
	Origen   string    `json:"origen"`
	CreadoEn time.Time `json:"creado_en"`
}
