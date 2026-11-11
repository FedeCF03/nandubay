package servicios

import (
	"context"
	"fmt"
	"strings"

	"github.com/FedeCF03/nandubay/backend/internal/modelos/acceso"
	"github.com/FedeCF03/nandubay/backend/internal/modelos/cortes"
	"github.com/FedeCF03/nandubay/backend/internal/modelos/muebles"
	"github.com/FedeCF03/nandubay/backend/internal/modelos/stock"
	"github.com/FedeCF03/nandubay/backend/internal/repositorios"
)

// ServicioCortes optimiza y maneja los cortes optimizados. Repo puede ser nil
// en los tests que solo optimizan.
type ServicioCortes struct {
	Repo *repositorios.RepoCortes
}

func (ServicioCortes) Optimizar(p cortes.Parametros, piezas []cortes.Pieza, mermaCantoMM int) (cortes.PlanCortes, error) {
	res, err := cortes.Optimizar(p, piezas)
	if err != nil {
		return cortes.PlanCortes{}, err
	}
	return cortes.PlanCortes{Resultado: res, MetrosCanto: cortes.MetrosCanto(piezas, mermaCantoMM)}, nil
}

func (s ServicioCortes) ListarCortes(ctx context.Context, u *acceso.Usuario, estado string) ([]cortes.CorteOptimizado, error) {
	return s.Repo.ListarCortes(ctx, u.CarpinteriaID, estado)
}

func (s ServicioCortes) VerCorte(ctx context.Context, u *acceso.Usuario, id int64) (cortes.CorteOptimizado, error) {
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
func (s ServicioCortes) CrearCorte(ctx context.Context, u *acceso.Usuario, n cortes.CorteNuevo) (cortes.CorteOptimizado, error) {
	n.Nombre = strings.TrimSpace(n.Nombre)
	if len([]rune(n.Nombre)) > 120 {
		return cortes.CorteOptimizado{}, stock.ErrValidacion{Campo: "nombre", Mensaje: "el nombre es muy largo"}
	}
	if len(n.Piezas) == 0 || len(n.Piezas) > 500 {
		return cortes.CorteOptimizado{}, stock.ErrValidacion{Campo: "piezas", Mensaje: "cargá las piezas a cortar"}
	}
	if n.DepositoID <= 0 {
		return cortes.CorteOptimizado{}, stock.ErrValidacion{Campo: "deposito_id", Mensaje: "elegí el depósito de donde salen las placas"}
	}
	if n.Kerf < 0 || n.Kerf > 20 {
		return cortes.CorteOptimizado{}, stock.ErrValidacion{Campo: "kerf", Mensaje: "el espesor de la sierra no es válido"}
	}
	ps, err := s.Repo.BuscarProductos(ctx, u.CarpinteriaID, []int64{n.PlacaID})
	if err != nil {
		return cortes.CorteOptimizado{}, err
	}
	placa, ok := ps[n.PlacaID]
	if !ok || placa.Tipo != stock.TipoPlaca || placa.LargoMM <= 0 || placa.AnchoMM <= 0 {
		return cortes.CorteOptimizado{}, stock.ErrValidacion{Campo: "placa_id", Mensaje: "elegí una placa del stock con medidas"}
	}
	par := cortes.Parametros{
		PlacaLargo: placa.LargoMM, PlacaAncho: placa.AnchoMM, TieneVeta: placa.TieneVeta, Kerf: n.Kerf, Refilado: placa.RefiladoMM,
		SobranteMinLargo: muebles.SobranteMinLargo, SobranteMinAncho: muebles.SobranteMinAncho,
	}
	res, err := s.optimizar(ctx, u, par, n.PlacaID, n.DepositoID, n.Piezas)
	if err != nil {
		return cortes.CorteOptimizado{}, err
	}
	id, err := s.Repo.CrearCorte(ctx, u.CarpinteriaID, u.ID, cortes.CorteOptimizado{
		Nombre: n.Nombre, PlacaID: n.PlacaID, DepositoID: n.DepositoID, Parametros: par, Piezas: n.Piezas, Resultado: res,
	})
	if err != nil {
		return cortes.CorteOptimizado{}, err
	}
	return s.VerCorte(ctx, u, id)
}

// Recalcular vuelve a optimizar un corte pendiente con los retazos que hay
// ahora (por ejemplo, si otro corte usó uno de los que tenía).
func (s ServicioCortes) Recalcular(ctx context.Context, u *acceso.Usuario, id int64) (cortes.CorteOptimizado, error) {
	c, err := s.Repo.BuscarCorte(ctx, u.CarpinteriaID, id)
	if err != nil {
		return c, err
	}
	if c.Estado != cortes.CortePendiente {
		return c, fmt.Errorf("%w: el corte ya está %s", cortes.ErrCorteEstado, c.Estado)
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

func (s ServicioCortes) ConfirmarCorte(ctx context.Context, u *acceso.Usuario, id int64) (cortes.CorteOptimizado, error) {
	if err := s.Repo.ConfirmarCorte(ctx, u.CarpinteriaID, u.ID, id); err != nil {
		return cortes.CorteOptimizado{}, err
	}
	return s.VerCorte(ctx, u, id)
}

func (s ServicioCortes) AnularCorte(ctx context.Context, u *acceso.Usuario, id int64) (cortes.CorteOptimizado, error) {
	if err := s.Repo.Anular(ctx, u.CarpinteriaID, id); err != nil {
		return cortes.CorteOptimizado{}, err
	}
	return s.VerCorte(ctx, u, id)
}
