package cortes

import (
	"errors"
	"time"
)

// PlanCortes es el resultado de optimizar, con los metros de canto.
type PlanCortes struct {
	Resultado
	MetrosCanto float64 `json:"metros_canto"`
}

// Estados de un corte optimizado.
const (
	CortePendiente = "pendiente"
	CorteCortado   = "cortado"
	CorteAnulado   = "anulado"
)

var (
	ErrCorteNoEncontrado = errors.New("no se encontró el corte")
	// ErrCorteEstado: el corte ya se cortó o se anuló, o un retazo ya no está.
	ErrCorteEstado = errors.New("no se puede hacer eso con el corte")
)

// CorteOptimizado es un plan de corte guardado de una placa del stock. Queda
// pendiente hasta que en el taller se confirma que se cortó: ahí se descuentan
// las placas enteras, se marcan los retazos usados y los sobrantes se guardan
// como retazos nuevos.
type CorteOptimizado struct {
	ID           int64      `json:"id"`
	Nombre       string     `json:"nombre"`
	PedidoID     *int64     `json:"pedido_id"`
	PedidoNumero *int64     `json:"pedido_numero"`
	Cliente      string     `json:"cliente"`
	PlacaID      int64      `json:"placa_id"`
	Placa        string     `json:"placa"`
	DepositoID   int64      `json:"deposito_id"`
	Deposito     string     `json:"deposito"`
	Estado       string     `json:"estado"`
	Parametros   Parametros `json:"parametros"`
	Piezas       []Pieza    `json:"piezas"`
	Resultado    Resultado  `json:"resultado"`
	MetrosCanto  float64    `json:"metros_canto"`
	CreadoEn     time.Time  `json:"creado_en"`
	CreadoPor    string     `json:"creado_por"`
	CortadoEn    *time.Time `json:"cortado_en"`
	CortadoPor   string     `json:"cortado_por"`
}

// CorteNuevo es lo que se manda para guardar un corte desde el optimizador.
type CorteNuevo struct {
	Nombre     string  `json:"nombre"`
	PlacaID    int64   `json:"placa_id"`
	DepositoID int64   `json:"deposito_id"`
	Kerf       int     `json:"kerf"`
	Piezas     []Pieza `json:"piezas"`
}
