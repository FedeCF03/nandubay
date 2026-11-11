package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/FedeCF03/nandubay/backend/internal/aplicacion"
	"github.com/FedeCF03/nandubay/backend/internal/dominio/cortes"
	"github.com/FedeCF03/nandubay/backend/internal/dominio/muebles"
	"github.com/FedeCF03/nandubay/backend/internal/dominio/stock"
	"github.com/jackc/pgx/v5"
)

func (r *RepoStock) ListarRetazos(ctx context.Context, carpinteriaID int64, f aplicacion.FiltroRetazos) ([]aplicacion.RetazoStock, error) {
	out := []aplicacion.RetazoStock{}
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
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
