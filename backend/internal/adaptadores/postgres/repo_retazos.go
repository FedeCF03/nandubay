package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/FedeCF03/nandubay/backend/internal/aplicacion"
	"github.com/FedeCF03/nandubay/backend/internal/dominio/cortes"
	"github.com/FedeCF03/nandubay/backend/internal/dominio/muebles"
	"github.com/FedeCF03/nandubay/backend/internal/dominio/stock"
	"github.com/FedeCF03/nandubay/backend/internal/dominio/ventas"
	"github.com/jackc/pgx/v5"
)

func (r *RepoStock) ListarRetazos(ctx context.Context, carpinteriaID int64, f aplicacion.FiltroRetazos) ([]aplicacion.RetazoStock, error) {
	out := []aplicacion.RetazoStock{}
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT r.id, r.placa_id, p.nombre, r.deposito_id, d.nombre, r.largo_mm, r.ancho_mm, coalesce(r.nota, ''),
			       coalesce('Venta N° ' || o.numero, ''), r.creado_en
			FROM retazos r
			JOIN productos p ON p.id = r.placa_id
			JOIN depositos d ON d.id = r.deposito_id
			LEFT JOIN pedidos o ON o.id = r.pedido_origen_id
			WHERE r.estado = 'disponible' AND ($1 = 0 OR r.placa_id = $1) AND ($2 = 0 OR r.deposito_id = $2)
			ORDER BY p.nombre, r.largo_mm * r.ancho_mm DESC, r.id`, f.PlacaID, f.DepositoID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var x aplicacion.RetazoStock
			if err := rows.Scan(&x.ID, &x.PlacaID, &x.Placa, &x.DepositoID, &x.Deposito, &x.Largo, &x.Ancho, &x.Nota,
				&x.Origen, &x.CreadoEn); err != nil {
				return err
			}
			out = append(out, x)
		}
		return rows.Err()
	})
	return out, err
}

func (r *RepoStock) CrearRetazo(ctx context.Context, carpinteriaID int64, x aplicacion.RetazoStock) (int64, error) {
	var id int64
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		// Las claves foráneas no pasan por RLS: el depósito tiene que ser de esta carpintería.
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM depositos WHERE id = $1 AND activo)`, x.DepositoID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return stock.ErrNoEncontrado
		}
		return tx.QueryRow(ctx, `
			INSERT INTO retazos (carpinteria_id, placa_id, deposito_id, largo_mm, ancho_mm, nota)
			VALUES ($1, $2, $3, $4, $5, nullif($6, '')) RETURNING id`,
			carpinteriaID, x.PlacaID, x.DepositoID, x.Largo, x.Ancho, x.Nota).Scan(&id)
	})
	return id, err
}

func (r *RepoStock) DescartarRetazo(ctx context.Context, carpinteriaID, id int64) error {
	return EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		t, err := tx.Exec(ctx, `UPDATE retazos SET estado = 'descartado', usado_en = now() WHERE id = $1 AND estado = 'disponible'`, id)
		if err == nil && t.RowsAffected() == 0 {
			err = stock.ErrNoEncontrado
		}
		return err
	})
}

func (r *RepoVentas) RetazosDisponibles(ctx context.Context, carpinteriaID int64, placaIDs []int64, depositoID int64) (map[int64][]cortes.Retazo, error) {
	out := map[int64][]cortes.Retazo{}
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
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
			var x cortes.Retazo
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

func (r *RepoVentas) BuscarMaterialesConfirmados(ctx context.Context, carpinteriaID, pedidoID int64) (*muebles.Materiales, error) {
	var crudo []byte
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT materiales FROM pedidos WHERE id = $1`, pedidoID).Scan(&crudo)
	})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && crudo == nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m muebles.Materiales
	if err := json.Unmarshal(crudo, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// usarRetazos guarda el plan confirmado, marca como usados los retazos que usa
// y guarda los sobrantes de cada placa como retazos nuevos del depósito.
func usarRetazos(ctx context.Context, tx pgx.Tx, carpinteriaID, pedidoID, depositoID int64, mat *muebles.Materiales) error {
	crudo, err := json.Marshal(mat)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE pedidos SET materiales = $2 WHERE id = $1`, pedidoID, crudo); err != nil {
		return err
	}
	for _, pc := range mat.Placas {
		for _, pl := range pc.Plan.Placas {
			if pl.RetazoID != "" {
				t, err := tx.Exec(ctx, `
					UPDATE retazos SET estado = 'usado', pedido_uso_id = $2, usado_en = now()
					WHERE id = $1::bigint AND estado = 'disponible' AND deposito_id = $3 AND placa_id = $4`,
					pl.RetazoID, pedidoID, depositoID, pc.ProductoID)
				if err != nil {
					return err
				}
				if t.RowsAffected() == 0 {
					return fmt.Errorf("%w: el retazo #%s de %s ya no está disponible en ese depósito; volvé a abrir el presupuesto",
						ventas.ErrEstado, pl.RetazoID, pc.Nombre)
				}
			}
			for _, s := range pl.Sobrantes {
				if _, err := tx.Exec(ctx, `
					INSERT INTO retazos (carpinteria_id, placa_id, deposito_id, largo_mm, ancho_mm, pedido_origen_id)
					VALUES ($1, $2, $3, $4, $5, $6)`,
					carpinteriaID, pc.ProductoID, depositoID, s.Largo, s.Ancho, pedidoID); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// devolverRetazos deshace usarRetazos al cancelar: los retazos usados vuelven a
// estar disponibles y los que generó la venta (si nadie los usó) se descartan.
func devolverRetazos(ctx context.Context, tx pgx.Tx, pedidoID int64) error {
	if _, err := tx.Exec(ctx, `
		UPDATE retazos SET estado = 'disponible', pedido_uso_id = NULL, usado_en = NULL
		WHERE pedido_uso_id = $1 AND estado = 'usado'`, pedidoID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		UPDATE retazos SET estado = 'descartado', usado_en = now()
		WHERE pedido_origen_id = $1 AND estado = 'disponible'`, pedidoID)
	return err
}
