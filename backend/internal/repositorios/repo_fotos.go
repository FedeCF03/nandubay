package repositorios

import (
	"context"
	"errors"

	"github.com/FedeCF03/nandubay/backend/internal/modelos/ventas"
	"github.com/jackc/pgx/v5"
)

func (r *RepoVentas) ListarFotos(ctx context.Context, carpinteriaID, pedidoID int64) ([]ventas.FotoMueble, error) {
	out := []ventas.FotoMueble{}
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, mueble_clave::text, creado_en FROM mueble_fotos
			WHERE pedido_id = $1 ORDER BY id`, pedidoID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var f ventas.FotoMueble
			if err := rows.Scan(&f.ID, &f.MuebleClave, &f.CreadoEn); err != nil {
				return err
			}
			out = append(out, f)
		}
		return rows.Err()
	})
	return out, err
}

func (r *RepoVentas) AgregarFoto(ctx context.Context, carpinteriaID, usuarioID, pedidoID int64, f ventas.FotoNueva) (ventas.FotoMueble, error) {
	out := ventas.FotoMueble{MuebleClave: f.MuebleClave}
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		// Bloquea el mueble para que dos subidas a la vez no pasen el máximo.
		var muebleID int64
		err := tx.QueryRow(ctx, `
			SELECT id FROM pedido_muebles WHERE pedido_id = $1 AND clave::text = $2 FOR UPDATE`,
			pedidoID, f.MuebleClave).Scan(&muebleID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ventas.ErrNoEncontrado
		}
		if err != nil {
			return err
		}
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM mueble_fotos WHERE pedido_id = $1 AND mueble_clave::text = $2`,
			pedidoID, f.MuebleClave).Scan(&n); err != nil {
			return err
		}
		if n >= ventas.MaxFotosPorMueble {
			return ventas.ErrDemasiadasFotos
		}
		return tx.QueryRow(ctx, `
			INSERT INTO mueble_fotos (carpinteria_id, pedido_id, mueble_clave, tipo, datos, usuario_id)
			VALUES ($1, $2, $3::uuid, $4, $5, $6) RETURNING id, creado_en`,
			carpinteriaID, pedidoID, f.MuebleClave, f.Tipo, f.Datos, usuarioID).Scan(&out.ID, &out.CreadoEn)
	})
	return out, err
}

func (r *RepoVentas) BuscarFoto(ctx context.Context, carpinteriaID, pedidoID, fotoID int64) (tipo string, datos []byte, err error) {
	err = EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `SELECT tipo, datos FROM mueble_fotos WHERE id = $1 AND pedido_id = $2`,
			fotoID, pedidoID).Scan(&tipo, &datos)
		if errors.Is(err, pgx.ErrNoRows) {
			return ventas.ErrNoEncontrado
		}
		return err
	})
	return
}

func (r *RepoVentas) BorrarFoto(ctx context.Context, carpinteriaID, pedidoID, fotoID int64) error {
	return EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		t, err := tx.Exec(ctx, `DELETE FROM mueble_fotos WHERE id = $1 AND pedido_id = $2`, fotoID, pedidoID)
		if err == nil && t.RowsAffected() == 0 {
			err = ventas.ErrNoEncontrado
		}
		return err
	})
}
