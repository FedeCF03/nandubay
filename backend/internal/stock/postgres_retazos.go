package stock

import (
	"context"

	"github.com/FedeCF03/nandubay/backend/internal/db"
	"github.com/jackc/pgx/v5"
)

func (r *PostgresStore) ListarRetazos(ctx context.Context, carpinteriaID int64, f FiltroRetazos) ([]RetazoStock, error) {
	out := []RetazoStock{}
	err := db.EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT r.id, r.placa_id, p.nombre, r.deposito_id, d.nombre, r.largo_mm, r.ancho_mm, coalesce(r.nota, ''),
			       coalesce('Venta N° ' || o.numero, 'Corte #' || r.plan_origen_id, ''), r.creado_en
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
			var x RetazoStock
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

func (r *PostgresStore) CrearRetazo(ctx context.Context, carpinteriaID int64, x RetazoStock) (int64, error) {
	var id int64
	err := db.EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		// Las claves foráneas no pasan por RLS: el depósito tiene que ser de esta carpintería.
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM depositos WHERE id = $1 AND activo)`, x.DepositoID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return ErrNoEncontrado
		}
		return tx.QueryRow(ctx, `
			INSERT INTO retazos (carpinteria_id, placa_id, deposito_id, largo_mm, ancho_mm, nota)
			VALUES ($1, $2, $3, $4, $5, nullif($6, '')) RETURNING id`,
			carpinteriaID, x.PlacaID, x.DepositoID, x.Largo, x.Ancho, x.Nota).Scan(&id)
	})
	return id, err
}

func (r *PostgresStore) DescartarRetazo(ctx context.Context, carpinteriaID, id int64) error {
	return db.EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		t, err := tx.Exec(ctx, `UPDATE retazos SET estado = 'descartado', usado_en = now() WHERE id = $1 AND estado = 'disponible'`, id)
		if err == nil && t.RowsAffected() == 0 {
			err = ErrNoEncontrado
		}
		return err
	})
}
