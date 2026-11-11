package aplicacion

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/FedeCF03/nandubay/backend/internal/dominio/acceso"
	"github.com/FedeCF03/nandubay/backend/internal/dominio/cortes"
	"github.com/FedeCF03/nandubay/backend/internal/dominio/muebles"
	"github.com/FedeCF03/nandubay/backend/internal/dominio/stock"
)

// PlanCortes es el resultado de optimizar, con los metros de canto.
type PlanCortes struct {
	cortes.Resultado
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
	ID           int64             `json:"id"`
	Nombre       string            `json:"nombre"`
	PedidoID     *int64            `json:"pedido_id"`
	PedidoNumero *int64            `json:"pedido_numero"`
	Cliente      string            `json:"cliente"`
	PlacaID      int64             `json:"placa_id"`
	Placa        string            `json:"placa"`
	DepositoID   int64             `json:"deposito_id"`
	Deposito     string            `json:"deposito"`
	Estado       string            `json:"estado"`
	Parametros   cortes.Parametros `json:"parametros"`
	Piezas       []cortes.Pieza    `json:"piezas"`
	Resultado    cortes.Resultado  `json:"resultado"`
	MetrosCanto  float64           `json:"metros_canto"`
	CreadoEn     time.Time         `json:"creado_en"`
	CreadoPor    string            `json:"creado_por"`
	CortadoEn    *time.Time        `json:"cortado_en"`
	CortadoPor   string            `json:"cortado_por"`
}

// CorteNuevo es lo que se manda para guardar un corte desde el optimizador.
type CorteNuevo struct {
	Nombre     string         `json:"nombre"`
	PlacaID    int64          `json:"placa_id"`
	DepositoID int64          `json:"deposito_id"`
	Kerf       int            `json:"kerf"`
	Piezas     []cortes.Pieza `json:"piezas"`
}

// RepositorioCortes guarda los cortes optimizados.
type RepositorioCortes interface {
	BuscarProductos(ctx context.Context, carpinteriaID int64, ids []int64) (map[int64]stock.Producto, error)
	RetazosDisponibles(ctx context.Context, carpinteriaID int64, placaIDs []int64, depositoID int64) (map[int64][]cortes.Retazo, error)
	ListarCortes(ctx context.Context, carpinteriaID int64, estado string) ([]CorteOptimizado, error)
	BuscarCorte(ctx context.Context, carpinteriaID, id int64) (CorteOptimizado, error)
	CrearCorte(ctx context.Context, carpinteriaID, usuarioID int64, c CorteOptimizado) (int64, error)
	// ActualizarResultado cambia el plan de un corte pendiente.
	ActualizarResultado(ctx context.Context, carpinteriaID, id int64, r cortes.Resultado) error
	Anular(ctx context.Context, carpinteriaID, id int64) error
	// ConfirmarCorte descuenta las placas, marca los retazos y guarda los
	// sobrantes, todo junto. Falla con ErrCorteEstado si algún retazo ya no está.
	ConfirmarCorte(ctx context.Context, carpinteriaID, usuarioID, id int64) error
}

// ServicioCortes optimiza y maneja los cortes optimizados. Repo puede ser nil
// en los tests que solo optimizan.
type ServicioCortes struct {
	Repo RepositorioCortes
}

func (ServicioCortes) Optimizar(p cortes.Parametros, piezas []cortes.Pieza, mermaCantoMM int) (PlanCortes, error) {
	res, err := cortes.Optimizar(p, piezas)
	if err != nil {
		return PlanCortes{}, err
	}
	return PlanCortes{res, cortes.MetrosCanto(piezas, mermaCantoMM)}, nil
}

func (s ServicioCortes) ListarCortes(ctx context.Context, u *acceso.Usuario, estado string) ([]CorteOptimizado, error) {
	return s.Repo.ListarCortes(ctx, u.CarpinteriaID, estado)
}

func (s ServicioCortes) VerCorte(ctx context.Context, u *acceso.Usuario, id int64) (CorteOptimizado, error) {
	c, err := s.Repo.BuscarCorte(ctx, u.CarpinteriaID, id)
	if err == nil {
		c.MetrosCanto = cortes.MetrosCanto(c.Piezas, muebles.MermaCantoMM)
	}
	return c, err
}

// optimizar arma el plan de una placa del stock con los retazos disponibles
// de ese depósito.
func (s ServicioCortes) optimizar(ctx context.Context, u *acceso.Usuario, par cortes.Parametros, placaID, depositoID int64, piezas []cortes.Pieza) (cortes.Resultado, error) {
	rs, err := s.Repo.RetazosDisponibles(ctx, u.CarpinteriaID, []int64{placaID}, depositoID)
	if err != nil {
		return cortes.Resultado{}, err
	}
	par.Retazos = rs[placaID]
	res, err := cortes.Optimizar(par, piezas)
	if err != nil {
		return res, stock.ErrValidacion{Campo: "piezas", Mensaje: err.Error()}
	}
	if len(res.NoColocadas) > 0 {
		return res, stock.ErrValidacion{Campo: "piezas", Mensaje: fmt.Sprintf("hay %d piezas que no entran en la placa", len(res.NoColocadas))}
	}
	return res, nil
}

// CrearCorte guarda un corte pendiente de una placa del stock, optimizado con
// los retazos del depósito.
func (s ServicioCortes) CrearCorte(ctx context.Context, u *acceso.Usuario, n CorteNuevo) (CorteOptimizado, error) {
	n.Nombre = strings.TrimSpace(n.Nombre)
	if len([]rune(n.Nombre)) > 120 {
		return CorteOptimizado{}, stock.ErrValidacion{Campo: "nombre", Mensaje: "el nombre es muy largo"}
	}
	if len(n.Piezas) == 0 || len(n.Piezas) > 500 {
		return CorteOptimizado{}, stock.ErrValidacion{Campo: "piezas", Mensaje: "cargá las piezas a cortar"}
	}
	if n.DepositoID <= 0 {
		return CorteOptimizado{}, stock.ErrValidacion{Campo: "deposito_id", Mensaje: "elegí el depósito de donde salen las placas"}
	}
	if n.Kerf < 0 || n.Kerf > 20 {
		return CorteOptimizado{}, stock.ErrValidacion{Campo: "kerf", Mensaje: "el espesor de la sierra no es válido"}
	}
	ps, err := s.Repo.BuscarProductos(ctx, u.CarpinteriaID, []int64{n.PlacaID})
	if err != nil {
		return CorteOptimizado{}, err
	}
	placa, ok := ps[n.PlacaID]
	if !ok || placa.Tipo != stock.TipoPlaca || placa.LargoMM <= 0 || placa.AnchoMM <= 0 {
		return CorteOptimizado{}, stock.ErrValidacion{Campo: "placa_id", Mensaje: "elegí una placa del stock con medidas"}
	}
	par := cortes.Parametros{
		PlacaLargo: placa.LargoMM, PlacaAncho: placa.AnchoMM, TieneVeta: placa.TieneVeta, Kerf: n.Kerf, Refilado: placa.RefiladoMM,
		SobranteMinLargo: muebles.SobranteMinLargo, SobranteMinAncho: muebles.SobranteMinAncho,
	}
	res, err := s.optimizar(ctx, u, par, n.PlacaID, n.DepositoID, n.Piezas)
	if err != nil {
		return CorteOptimizado{}, err
	}
	id, err := s.Repo.CrearCorte(ctx, u.CarpinteriaID, u.ID, CorteOptimizado{
		Nombre: n.Nombre, PlacaID: n.PlacaID, DepositoID: n.DepositoID, Parametros: par, Piezas: n.Piezas, Resultado: res,
	})
	if err != nil {
		return CorteOptimizado{}, err
	}
	return s.VerCorte(ctx, u, id)
}

// Recalcular vuelve a optimizar un corte pendiente con los retazos que hay
// ahora (por ejemplo, si otro corte usó uno de los que tenía).
func (s ServicioCortes) Recalcular(ctx context.Context, u *acceso.Usuario, id int64) (CorteOptimizado, error) {
	c, err := s.Repo.BuscarCorte(ctx, u.CarpinteriaID, id)
	if err != nil {
		return c, err
	}
	if c.Estado != CortePendiente {
		return c, fmt.Errorf("%w: el corte ya está %s", ErrCorteEstado, c.Estado)
	}
	res, err := s.optimizar(ctx, u, c.Parametros, c.PlacaID, c.DepositoID, c.Piezas)
	if err != nil {
		return c, err
	}
	if err := s.Repo.ActualizarResultado(ctx, u.CarpinteriaID, id, res); err != nil {
		return c, err
	}
	return s.VerCorte(ctx, u, id)
}

func (s ServicioCortes) ConfirmarCorte(ctx context.Context, u *acceso.Usuario, id int64) (CorteOptimizado, error) {
	if err := s.Repo.ConfirmarCorte(ctx, u.CarpinteriaID, u.ID, id); err != nil {
		return CorteOptimizado{}, err
	}
	return s.VerCorte(ctx, u, id)
}

func (s ServicioCortes) AnularCorte(ctx context.Context, u *acceso.Usuario, id int64) (CorteOptimizado, error) {
	if err := s.Repo.Anular(ctx, u.CarpinteriaID, id); err != nil {
		return CorteOptimizado{}, err
	}
	return s.VerCorte(ctx, u, id)
}
