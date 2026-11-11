package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/FedeCF03/nandubay/backend/internal/model/cortes"
	"github.com/FedeCF03/nandubay/backend/internal/model/muebles"
	"github.com/FedeCF03/nandubay/backend/internal/model/stock"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Cortes guarda los cortes optimizados (tabla planes_corte).
type Cortes struct {
	DB *pgxpool.Pool
}

func (r *Cortes) BuscarProductos(ctx context.Context, carpinteriaID int64, ids []int64) (map[int64]stock.Producto, error) {
	return (&Ventas{DB: r.DB}).BuscarProductos(ctx, carpinteriaID, ids)
}

func (r *Cortes) RetazosDisponibles(ctx context.Context, carpinteriaID int64, placaIDs []int64, depositoID int64) (map[int64][]cortes.Retazo, error) {
	return (&Ventas{DB: r.DB}).RetazosDisponibles(ctx, carpinteriaID, placaIDs, depositoID)
}

const sqlCorte = `
	SELECT c.id, c.nombre, c.pedido_id, o.numero, coalesce(cl.nombre, ''), c.placa_id, p.nombre,
	       coalesce(c.deposito_id, 0), coalesce(d.nombre, ''), c.estado, c.parametros, c.piezas, c.resultado,
	       c.creado_en, coalesce(uc.nombre, ''), c.cortado_en, coalesce(uk.nombre, '')
	FROM planes_corte c
	JOIN productos p ON p.id = c.placa_id
	LEFT JOIN depositos d ON d.id = c.deposito_id
	LEFT JOIN pedidos o ON o.id = c.pedido_id
	LEFT JOIN clientes cl ON cl.id = o.cliente_id
	LEFT JOIN usuarios uc ON uc.id = c.creado_por
	LEFT JOIN usuarios uk ON uk.id = c.cortado_por`

func escanearCorte(row pgx.Row) (cortes.CorteOptimizado, error) {
	var c cortes.CorteOptimizado
	var numero *int32
	var par, pzs, res []byte
	err := row.Scan(&c.ID, &c.Nombre, &c.PedidoID, &numero, &c.Cliente, &c.PlacaID, &c.Placa, &c.DepositoID, &c.Deposito,
		&c.Estado, &par, &pzs, &res, &c.CreadoEn, &c.CreadoPor, &c.CortadoEn, &c.CortadoPor)
	if err != nil {
		return c, err
	}
	if numero != nil {
		n := int64(*numero)
		c.PedidoNumero = &n
	}
	if err := json.Unmarshal(par, &c.Parametros); err != nil {
		return c, err
	}
	if err := json.Unmarshal(pzs, &c.Piezas); err != nil {
		return c, err
	}
	return c, json.Unmarshal(res, &c.Resultado)
}

func (r *Cortes) ListarCortes(ctx context.Context, carpinteriaID int64, estado string) ([]cortes.CorteOptimizado, error) {
	out := []cortes.CorteOptimizado{}
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, sqlCorte+`
			WHERE c.deposito_id IS NOT NULL AND ($1 = '' OR c.estado = $1)
			ORDER BY (c.estado = 'pendiente') DESC, c.creado_en DESC, c.id DESC
			LIMIT 300`, estado)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			c, err := escanearCorte(rows)
			if err != nil {
				return err
			}
			// En la lista no hacen falta las posiciones de cada pieza.
			for i := range c.Resultado.Placas {
				c.Resultado.Placas[i].Piezas = nil
				c.Resultado.Placas[i].Sobrantes = nil
			}
			c.Piezas = nil
			out = append(out, c)
		}
		return rows.Err()
	})
	return out, err
}

func (r *Cortes) BuscarCorte(ctx context.Context, carpinteriaID, id int64) (cortes.CorteOptimizado, error) {
	var c cortes.CorteOptimizado
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		var err error
		c, err = escanearCorte(tx.QueryRow(ctx, sqlCorte+` WHERE c.id = $1 AND c.deposito_id IS NOT NULL`, id))
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return c, cortes.ErrCorteNoEncontrado
	}
	return c, err
}

func (r *Cortes) CrearCorte(ctx context.Context, carpinteriaID, usuarioID int64, c cortes.CorteOptimizado) (int64, error) {
	var id int64
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM depositos WHERE id = $1 AND activo)`, c.DepositoID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return stock.ErrNoEncontrado
		}
		var err error
		id, err = insertarCorte(ctx, tx, carpinteriaID, usuarioID, nil, c.DepositoID, c.Nombre, muebles.PlanDeCorte{
			PlacaID: c.PlacaID, Parametros: c.Parametros, Piezas: c.Piezas, Resultado: c.Resultado,
		})
		return err
	})
	return id, err
}

func insertarCorte(ctx context.Context, tx pgx.Tx, carpinteriaID, usuarioID int64, pedidoID *int64, depositoID int64, nombre string, p muebles.PlanDeCorte) (int64, error) {
	par, err := json.Marshal(p.Parametros)
	if err != nil {
		return 0, err
	}
	pzs, err := json.Marshal(p.Piezas)
	if err != nil {
		return 0, err
	}
	res, err := json.Marshal(p.Resultado)
	if err != nil {
		return 0, err
	}
	var id int64
	err = tx.QueryRow(ctx, `
		INSERT INTO planes_corte (carpinteria_id, pedido_id, placa_id, deposito_id, nombre, kerf_mm, refilado_mm,
		                          aprovechamiento, parametros, piezas, resultado, creado_por)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING id`,
		carpinteriaID, pedidoID, p.PlacaID, depositoID, nombre, p.Parametros.Kerf, p.Parametros.Refilado,
		p.Resultado.Aprovechamiento, par, pzs, res, usuarioID).Scan(&id)
	return id, err
}

func (r *Cortes) ActualizarResultado(ctx context.Context, carpinteriaID, id int64, res cortes.Resultado) error {
	crudo, err := json.Marshal(res)
	if err != nil {
		return err
	}
	return EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		t, err := tx.Exec(ctx, `UPDATE planes_corte SET resultado = $2, aprovechamiento = $3 WHERE id = $1 AND estado = 'pendiente'`,
			id, crudo, res.Aprovechamiento)
		if err == nil && t.RowsAffected() == 0 {
			err = fmt.Errorf("%w: el corte ya no está pendiente", cortes.ErrCorteEstado)
		}
		return err
	})
}

func (r *Cortes) Anular(ctx context.Context, carpinteriaID, id int64) error {
	return EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		var estado string
		err := tx.QueryRow(ctx, `SELECT estado FROM planes_corte WHERE id = $1 AND deposito_id IS NOT NULL FOR UPDATE`, id).Scan(&estado)
		if errors.Is(err, pgx.ErrNoRows) {
			return cortes.ErrCorteNoEncontrado
		}
		if err != nil {
			return err
		}
		if estado != cortes.CortePendiente {
			return fmt.Errorf("%w: el corte ya está %s", cortes.ErrCorteEstado, estado)
		}
		_, err = tx.Exec(ctx, `UPDATE planes_corte SET estado = 'anulado' WHERE id = $1`, id)
		return err
	})
}

// ConfirmarCorte registra que el corte se hizo: descuenta las placas enteras
// del depósito, marca los retazos usados y guarda los sobrantes como retazos.
func (r *Cortes) ConfirmarCorte(ctx context.Context, carpinteriaID, usuarioID, id int64) error {
	return EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		c, err := escanearCorte(tx.QueryRow(ctx, sqlCorte+` WHERE c.id = $1 AND c.deposito_id IS NOT NULL FOR UPDATE OF c`, id))
		if errors.Is(err, pgx.ErrNoRows) {
			return cortes.ErrCorteNoEncontrado
		}
		if err != nil {
			return err
		}
		if c.Estado != cortes.CortePendiente {
			return fmt.Errorf("%w: el corte ya está %s", cortes.ErrCorteEstado, c.Estado)
		}
		nota := "Corte #" + strconv.FormatInt(c.ID, 10)
		if c.PedidoNumero != nil {
			nota += fmt.Sprintf(" (venta N° %d)", *c.PedidoNumero)
		}
		if n := c.Resultado.PlacasEnteras; n > 0 {
			op, err := stock.Corte(c.PlacaID, c.DepositoID, float64(n), nota)
			if err != nil {
				return err
			}
			if err := aplicarOperaciones(ctx, tx, carpinteriaID, usuarioID, c.PedidoID, []stock.Operacion{op}); err != nil {
				return err
			}
		}
		for _, pl := range c.Resultado.Placas {
			if pl.RetazoID != "" {
				t, err := tx.Exec(ctx, `
					UPDATE retazos SET estado = 'usado', plan_uso_id = $2, pedido_uso_id = $5, usado_en = now()
					WHERE id = $1::bigint AND estado = 'disponible' AND deposito_id = $3 AND placa_id = $4`,
					pl.RetazoID, c.ID, c.DepositoID, c.PlacaID, c.PedidoID)
				if err != nil {
					return err
				}
				if t.RowsAffected() == 0 {
					return fmt.Errorf("%w: el retazo #%s ya no está disponible; tocá \"Recalcular\" para armar el plan de nuevo",
						cortes.ErrCorteEstado, pl.RetazoID)
				}
			}
			for _, s := range pl.Sobrantes {
				if _, err := tx.Exec(ctx, `
					INSERT INTO retazos (carpinteria_id, placa_id, deposito_id, largo_mm, ancho_mm, plan_origen_id, pedido_origen_id)
					VALUES ($1, $2, $3, $4, $5, $6, $7)`,
					carpinteriaID, c.PlacaID, c.DepositoID, s.Largo, s.Ancho, c.ID, c.PedidoID); err != nil {
					return err
				}
			}
		}
		_, err = tx.Exec(ctx, `UPDATE planes_corte SET estado = 'cortado', cortado_en = now(), cortado_por = $2 WHERE id = $1`, id, usuarioID)
		return err
	})
}

// guardarMateriales guarda el plan con que se confirmó un presupuesto de
// muebles y deja un corte pendiente por cada placa.
func guardarMateriales(ctx context.Context, tx pgx.Tx, carpinteriaID, usuarioID, pedidoID, depositoID int64, mat *muebles.Materiales) error {
	crudo, err := json.Marshal(mat)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE pedidos SET materiales = $2 WHERE id = $1`, pedidoID, crudo); err != nil {
		return err
	}
	for _, p := range mat.Cortes {
		if _, err := insertarCorte(ctx, tx, carpinteriaID, usuarioID, &pedidoID, depositoID, "", p); err != nil {
			return err
		}
	}
	return nil
}

// anularCortesPendientes: al cancelar una venta, sus cortes sin hacer se
// anulan. Los que ya se cortaron quedan como están (la madera ya se usó).
func anularCortesPendientes(ctx context.Context, tx pgx.Tx, pedidoID int64) error {
	_, err := tx.Exec(ctx, `UPDATE planes_corte SET estado = 'anulado' WHERE pedido_id = $1 AND estado = 'pendiente'`, pedidoID)
	return err
}
