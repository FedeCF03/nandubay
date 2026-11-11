package cortes

import (
	"context"
	"fmt"
	"strings"

	"github.com/FedeCF03/nandubay/backend/internal/acceso"
	"github.com/FedeCF03/nandubay/backend/internal/stock"
)

// Servicio optimiza y maneja los cortes optimizados. Store puede ser nil
// en los tests que solo optimizan.
type Servicio struct {
	Store Store
}

func (Servicio) Optimizar(p Parametros, piezas []Pieza, mermaCantoMM int) (PlanCortes, error) {
	res, err := Optimizar(p, piezas)
	if err != nil {
		return PlanCortes{}, err
	}
	return PlanCortes{Resultado: res, MetrosCanto: MetrosCanto(piezas, mermaCantoMM)}, nil
}

func (s Servicio) ListarCortes(ctx context.Context, u *acceso.Usuario, estado string) ([]CorteOptimizado, error) {
	return s.Store.ListarCortes(ctx, u.CarpinteriaID, estado)
}

func (s Servicio) VerCorte(ctx context.Context, u *acceso.Usuario, id int64) (CorteOptimizado, error) {
	c, err := s.Store.BuscarCorte(ctx, u.CarpinteriaID, id)
	if err == nil {
		c.MetrosCanto = MetrosCanto(c.Piezas, MermaCantoMM)
	}
	return c, err
}

// optimizar arma el plan de una placa del stock con los retazos disponibles
// de ese depósito.
func (s Servicio) optimizar(ctx context.Context, u *acceso.Usuario, par Parametros, placaID, depositoID int64, piezas []Pieza) (Resultado, error) {
	rs, err := s.Store.RetazosDisponibles(ctx, u.CarpinteriaID, []int64{placaID}, depositoID)
	if err != nil {
		return Resultado{}, err
	}
	par.Retazos = rs[placaID]
	res, err := Optimizar(par, piezas)
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
func (s Servicio) CrearCorte(ctx context.Context, u *acceso.Usuario, n CorteNuevo) (CorteOptimizado, error) {
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
	ps, err := s.Store.BuscarProductos(ctx, u.CarpinteriaID, []int64{n.PlacaID})
	if err != nil {
		return CorteOptimizado{}, err
	}
	placa, ok := ps[n.PlacaID]
	if !ok || placa.Tipo != stock.TipoPlaca || placa.LargoMM <= 0 || placa.AnchoMM <= 0 {
		return CorteOptimizado{}, stock.ErrValidacion{Campo: "placa_id", Mensaje: "elegí una placa del stock con medidas"}
	}
	par := Parametros{
		PlacaLargo: placa.LargoMM, PlacaAncho: placa.AnchoMM, TieneVeta: placa.TieneVeta, Kerf: n.Kerf, Refilado: placa.RefiladoMM,
		SobranteMinLargo: SobranteMinLargo, SobranteMinAncho: SobranteMinAncho,
	}
	res, err := s.optimizar(ctx, u, par, n.PlacaID, n.DepositoID, n.Piezas)
	if err != nil {
		return CorteOptimizado{}, err
	}
	id, err := s.Store.CrearCorte(ctx, u.CarpinteriaID, u.ID, CorteOptimizado{
		Nombre: n.Nombre, PlacaID: n.PlacaID, DepositoID: n.DepositoID, Parametros: par, Piezas: n.Piezas, Resultado: res,
	})
	if err != nil {
		return CorteOptimizado{}, err
	}
	return s.VerCorte(ctx, u, id)
}

// Recalcular vuelve a optimizar un corte pendiente con los retazos que hay
// ahora (por ejemplo, si otro corte usó uno de los que tenía).
func (s Servicio) Recalcular(ctx context.Context, u *acceso.Usuario, id int64) (CorteOptimizado, error) {
	c, err := s.Store.BuscarCorte(ctx, u.CarpinteriaID, id)
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
	if err := s.Store.ActualizarResultado(ctx, u.CarpinteriaID, id, res); err != nil {
		return c, err
	}
	return s.VerCorte(ctx, u, id)
}

func (s Servicio) ConfirmarCorte(ctx context.Context, u *acceso.Usuario, id int64) (CorteOptimizado, error) {
	if err := s.Store.ConfirmarCorte(ctx, u.CarpinteriaID, u.ID, id); err != nil {
		return CorteOptimizado{}, err
	}
	return s.VerCorte(ctx, u, id)
}

func (s Servicio) AnularCorte(ctx context.Context, u *acceso.Usuario, id int64) (CorteOptimizado, error) {
	if err := s.Store.Anular(ctx, u.CarpinteriaID, id); err != nil {
		return CorteOptimizado{}, err
	}
	return s.VerCorte(ctx, u, id)
}
