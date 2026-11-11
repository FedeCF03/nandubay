package cortes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/FedeCF03/nandubay/backend/internal/db"
	"github.com/FedeCF03/nandubay/backend/internal/stock"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore guarda los cortes optimizados (tabla planes_corte).
type PostgresStore struct {
	DB *pgxpool.Pool
}

func (r *PostgresStore) BuscarProductos(ctx context.Context, carpinteriaID int64, ids []int64) (map[int64]stock.Producto, error) {
	return (&stock.PostgresStore{DB: r.DB}).BuscarProductos(ctx, carpinteriaID, ids)
}

func (r *PostgresStore) RetazosDisponibles(ctx context.Context, carpinteriaID int64, placaIDs []int64, depositoID int64) (map[int64][]Retazo, error) {
	out := map[int64][]Retazo{}
	err := db.EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, placa_id, largo_mm, ancho_mm FROM retazos
			WHERE estado = 'disponible' AND placa_id = ANY($1) AND ($2 = 0 OR deposito_id = $2)
			ORDER BY id`, placaIDs, depositoID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id, placa int64
			var x Retazo
			if err := rows.Scan(&id, &placa, &x.Largo, &x.Ancho); err != nil {
				return err
			}
			x.ID = strconv.FormatInt(id, 10)
			out[placa] = append(out[placa], x)
		}
		return rows.Err()
	})
	return out, err
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

func escanearCorte(row pgx.Row) (CorteOptimizado, error) {
	var c CorteOptimizado
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

func (r *PostgresStore) ListarCortes(ctx context.Context, carpinteriaID int64, estado string) ([]CorteOptimizado, error) {
	out := []CorteOptimizado{}
	err := db.EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
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

func (r *PostgresStore) BuscarCorte(ctx context.Context, carpinteriaID, id int64) (CorteOptimizado, error) {
	var c CorteOptimizado
	err := db.EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		var err error
		c, err = escanearCorte(tx.QueryRow(ctx, sqlCorte+` WHERE c.id = $1 AND c.deposito_id IS NOT NULL`, id))
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrCorteNoEncontrado
	}
	return c, err
}

func (r *PostgresStore) CrearCorte(ctx context.Context, carpinteriaID, usuarioID int64, c CorteOptimizado) (int64, error) {
	var id int64
	err := db.EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM depositos WHERE id = $1 AND activo)`, c.DepositoID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return stock.ErrNoEncontrado
		}
		var err error
		id, err = InsertarCorte(ctx, tx, carpinteriaID, usuarioID, nil, c.DepositoID, c.Nombre, PlanDeCorte{
			PlacaID: c.PlacaID, Parametros: c.Parametros, Piezas: c.Piezas, Resultado: c.Resultado,
		})
		return err
	})
	return id, err
}

func InsertarCorte(ctx context.Context, tx pgx.Tx, carpinteriaID, usuarioID int64, pedidoID *int64, depositoID int64, nombre string, p PlanDeCorte) (int64, error) {
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

func (r *PostgresStore) ActualizarResultado(ctx context.Context, carpinteriaID, id int64, res Resultado) error {
	crudo, err := json.Marshal(res)
	if err != nil {
		return err
	}
	return db.EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		t, err := tx.Exec(ctx, `UPDATE planes_corte SET resultado = $2, aprovechamiento = $3 WHERE id = $1 AND estado = 'pendiente'`,
			id, crudo, res.Aprovechamiento)
		if err == nil && t.RowsAffected() == 0 {
			err = fmt.Errorf("%w: el corte ya no está pendiente", ErrCorteEstado)
		}
		return err
	})
}

func (r *PostgresStore) Anular(ctx context.Context, carpinteriaID, id int64) error {
	return db.EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		var estado string
		err := tx.QueryRow(ctx, `SELECT estado FROM planes_corte WHERE id = $1 AND deposito_id IS NOT NULL FOR UPDATE`, id).Scan(&estado)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrCorteNoEncontrado
		}
		if err != nil {
			return err
		}
		if estado != CortePendiente {
			return fmt.Errorf("%w: el corte ya está %s", ErrCorteEstado, estado)
		}
		_, err = tx.Exec(ctx, `UPDATE planes_corte SET estado = 'anulado' WHERE id = $1`, id)
		return err
	})
}

// ConfirmarCorte registra que el corte se hizo: descuenta las placas enteras
// del depósito, marca los retazos usados y guarda los sobrantes como retazos.
func (r *PostgresStore) ConfirmarCorte(ctx context.Context, carpinteriaID, usuarioID, id int64) error {
	return db.EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		c, err := escanearCorte(tx.QueryRow(ctx, sqlCorte+` WHERE c.id = $1 AND c.deposito_id IS NOT NULL FOR UPDATE OF c`, id))
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrCorteNoEncontrado
		}
		if err != nil {
			return err
		}
		if c.Estado != CortePendiente {
			return fmt.Errorf("%w: el corte ya está %s", ErrCorteEstado, c.Estado)
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
			if err := stock.AplicarOperaciones(ctx, tx, carpinteriaID, usuarioID, c.PedidoID, []stock.Operacion{op}); err != nil {
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
						ErrCorteEstado, pl.RetazoID)
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
