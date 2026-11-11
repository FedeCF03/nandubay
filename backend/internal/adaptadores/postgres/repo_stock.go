package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/FedeCF03/nandubay/backend/internal/aplicacion"
	"github.com/FedeCF03/nandubay/backend/internal/dominio/stock"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepoStock implementa aplicacion.RepositorioStock.
type RepoStock struct {
	DB *pgxpool.Pool
}

var _ aplicacion.RepositorioStock = (*RepoStock)(nil)

const columnasProducto = `p.id, p.tipo, coalesce(p.sku, ''), p.nombre, coalesce(p.marca, ''), coalesce(p.color, ''),
	p.unidad, p.costo::float8, p.precio_venta::float8, p.stock_minimo::float8,
	coalesce(p.largo_mm, 0), coalesce(p.ancho_mm, 0), coalesce(p.espesor_mm, 0)::float8, p.tiene_veta,
	p.refilado_mm, coalesce(p.canto_ancho_mm, 0), coalesce(p.canto_espesor_mm, 0)::float8, p.activo,
	p.categoria_id, coalesce((SELECT c.nombre FROM categorias_producto c WHERE c.id = p.categoria_id), '')`

func escanearProducto(row pgx.Row, p *stock.Producto, extra ...any) error {
	dest := []any{&p.ID, &p.Tipo, &p.Sku, &p.Nombre, &p.Marca, &p.Color, &p.Unidad, &p.Costo, &p.PrecioVenta,
		&p.StockMinimo, &p.LargoMM, &p.AnchoMM, &p.EspesorMM, &p.TieneVeta, &p.RefiladoMM, &p.CantoAnchoMM,
		&p.CantoEspMM, &p.Activo, &p.CategoriaID, &p.Categoria}
	return row.Scan(append(dest, extra...)...)
}

func (r *RepoStock) ListarProductos(ctx context.Context, carpinteriaID int64, f aplicacion.FiltroProductos) ([]aplicacion.ProductoConStock, error) {
	var out []aplicacion.ProductoConStock
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		conds := []string{"true"}
		args := []any{}
		if !f.Inactivos {
			conds = append(conds, "p.activo")
		}
		if f.Tipo != "" {
			args = append(args, f.Tipo)
			conds = append(conds, fmt.Sprintf("p.tipo = $%d::tipo_producto", len(args)))
		}
		if f.CategoriaID == -1 {
			conds = append(conds, "p.categoria_id IS NULL")
		} else if f.CategoriaID > 0 {
			args = append(args, f.CategoriaID)
			conds = append(conds, fmt.Sprintf("p.categoria_id = $%d", len(args)))
		}
		if b := strings.TrimSpace(f.Buscar); b != "" {
			args = append(args, "%"+b+"%")
			conds = append(conds, fmt.Sprintf("(p.nombre ILIKE $%[1]d OR p.sku ILIKE $%[1]d OR p.color ILIKE $%[1]d OR p.marca ILIKE $%[1]d)", len(args)))
		}
		rows, err := tx.Query(ctx, `
			SELECT `+columnasProducto+`,
			       coalesce(jsonb_object_agg(s.deposito_id, s.cantidad::float8) FILTER (WHERE s.deposito_id IS NOT NULL), '{}'),
			       coalesce(sum(s.cantidad), 0)::float8
			FROM productos p
			LEFT JOIN stock s ON s.producto_id = p.id
			WHERE `+strings.Join(conds, " AND ")+`
			GROUP BY p.id
			ORDER BY p.tipo, p.nombre`, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var p aplicacion.ProductoConStock
			if err := escanearProducto(rows, &p.Producto, &p.PorDeposito, &p.Total); err != nil {
				return err
			}
			out = append(out, p)
		}
		return rows.Err()
	})
	return out, err
}

func (r *RepoStock) BuscarProducto(ctx context.Context, carpinteriaID, id int64) (stock.Producto, error) {
	var p stock.Producto
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		return escanearProducto(tx.QueryRow(ctx, `SELECT `+columnasProducto+` FROM productos p WHERE p.id = $1`, id), &p)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return p, stock.ErrNoEncontrado
	}
	return p, err
}

func nulo[T comparable](v T) *T {
	var cero T
	if v == cero {
		return nil
	}
	return &v
}

func argsProducto(p stock.Producto) []any {
	return []any{p.Tipo, nulo(p.Sku), p.Nombre, nulo(p.Marca), nulo(p.Color), p.Unidad, p.Costo, p.PrecioVenta,
		p.StockMinimo, nulo(p.LargoMM), nulo(p.AnchoMM), nulo(p.EspesorMM), p.TieneVeta, p.RefiladoMM,
		nulo(p.CantoAnchoMM), nulo(p.CantoEspMM), p.Activo, p.CategoriaID}
}

func errorProducto(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "sku") {
		return stock.ErrSkuEnUso
	}
	// La categoría no existe o es de otra carpintería.
	if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == "productos_categoria_fk" {
		return stock.ErrValidacion{Campo: "categoria_id", Mensaje: "la categoría elegida no existe"}
	}
	return err
}

func (r *RepoStock) CrearProducto(ctx context.Context, carpinteriaID int64, p stock.Producto) (int64, error) {
	var id int64
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO productos (carpinteria_id, tipo, sku, nombre, marca, color, unidad, costo, precio_venta,
				stock_minimo, largo_mm, ancho_mm, espesor_mm, tiene_veta, refilado_mm, canto_ancho_mm, canto_espesor_mm, activo, categoria_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
			RETURNING id`, append([]any{carpinteriaID}, argsProducto(p)...)...).Scan(&id)
	})
	return id, errorProducto(err)
}

func (r *RepoStock) ActualizarProducto(ctx context.Context, carpinteriaID int64, p stock.Producto) error {
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE productos SET tipo = $2, sku = $3, nombre = $4, marca = $5, color = $6, unidad = $7, costo = $8,
				precio_venta = $9, stock_minimo = $10, largo_mm = $11, ancho_mm = $12, espesor_mm = $13,
				tiene_veta = $14, refilado_mm = $15, canto_ancho_mm = $16, canto_espesor_mm = $17, activo = $18,
				categoria_id = $19
			WHERE id = $1`, append([]any{p.ID}, argsProducto(p)...)...)
		if err == nil && tag.RowsAffected() == 0 {
			return stock.ErrNoEncontrado
		}
		return err
	})
	return errorProducto(err)
}

func (r *RepoStock) ListarDepositos(ctx context.Context, carpinteriaID int64) ([]stock.Deposito, error) {
	var out []stock.Deposito
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT d.id, d.nombre, s.id, s.nombre FROM depositos d
			JOIN sucursales s ON s.id = d.sucursal_id
			WHERE d.activo AND s.activa ORDER BY s.nombre, d.nombre`)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, pgx.RowToStructByPos[stock.Deposito])
		return err
	})
	return out, err
}

func (r *RepoStock) RegistrarOperaciones(ctx context.Context, carpinteriaID, usuarioID int64, ops []stock.Operacion) error {
	return EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		return aplicarOperaciones(ctx, tx, carpinteriaID, usuarioID, nil, ops)
	})
}

// aplicarOperaciones mueve el stock dentro de una transacción ya abierta, con
// cada saldo bloqueado. pedidoID queda en el movimiento si es de una venta.
func aplicarOperaciones(ctx context.Context, tx pgx.Tx, carpinteriaID, usuarioID int64, pedidoID *int64, ops []stock.Operacion) error {
	for _, op := range ops {
		// Las claves foráneas no pasan por RLS: hay que confirmar que el
		// depósito es de esta carpintería antes de usarlo.
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM depositos WHERE id = $1 AND activo)`, op.DepositoID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return stock.ErrNoEncontrado
		}
		// Crear la fila de saldo si no existe y bloquearla.
		if _, err := tx.Exec(ctx, `
				INSERT INTO stock (carpinteria_id, producto_id, deposito_id, cantidad) VALUES ($1, $2, $3, 0)
				ON CONFLICT (producto_id, deposito_id) DO NOTHING`, carpinteriaID, op.ProductoID, op.DepositoID); err != nil {
			return err
		}
		var saldo float64
		if err := tx.QueryRow(ctx, `
				SELECT cantidad::float8 FROM stock WHERE producto_id = $1 AND deposito_id = $2 FOR UPDATE`,
			op.ProductoID, op.DepositoID).Scan(&saldo); err != nil {
			return err
		}
		nuevo, err := op.Nuevo(saldo)
		if err != nil {
			// Decir de qué producto se trata: en una venta puede haber varios.
			var nombre string
			if errors.Is(err, stock.ErrStockInsuficiente) &&
				tx.QueryRow(ctx, `SELECT nombre FROM productos WHERE id = $1`, op.ProductoID).Scan(&nombre) == nil {
				return fmt.Errorf("%s: %w", nombre, err)
			}
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE stock SET cantidad = $3 WHERE producto_id = $1 AND deposito_id = $2`,
			op.ProductoID, op.DepositoID, nuevo); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
				INSERT INTO stock_movimientos (carpinteria_id, producto_id, deposito_id, tipo, cantidad, costo_unitario, usuario_id, nota, pedido_id)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			carpinteriaID, op.ProductoID, op.DepositoID, op.Tipo, nuevo-saldo, op.CostoUnitario, usuarioID, nulo(op.Nota), pedidoID); err != nil {
			return err
		}
		// Una compra actualiza el costo del producto.
		if op.Tipo == stock.MovCompra && op.CostoUnitario != nil {
			if _, err := tx.Exec(ctx, `UPDATE productos SET costo = $2 WHERE id = $1`, op.ProductoID, *op.CostoUnitario); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *RepoStock) ListarMovimientos(ctx context.Context, carpinteriaID, productoID int64, limite int) ([]aplicacion.MovimientoRegistrado, error) {
	var out []aplicacion.MovimientoRegistrado
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT m.id, m.producto_id, m.deposito_id, m.tipo, m.cantidad::float8, m.costo_unitario::float8,
			       coalesce(m.nota, ''), d.nombre || ' (' || s.nombre || ')', coalesce(u.nombre, ''), m.creado_en
			FROM stock_movimientos m
			JOIN depositos d ON d.id = m.deposito_id
			JOIN sucursales s ON s.id = d.sucursal_id
			LEFT JOIN usuarios u ON u.id = m.usuario_id
			WHERE m.producto_id = $1
			ORDER BY m.creado_en DESC, m.id DESC
			LIMIT $2`, productoID, limite)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var m aplicacion.MovimientoRegistrado
			if err := rows.Scan(&m.ID, &m.ProductoID, &m.DepositoID, &m.Tipo, &m.Cantidad, &m.CostoUnitario,
				&m.Nota, &m.Deposito, &m.Usuario, &m.Fecha); err != nil {
				return err
			}
			out = append(out, m)
		}
		return rows.Err()
	})
	return out, err
}

func errorCategoria(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return stock.ErrCategoriaEnUso
	}
	return err
}

func (r *RepoStock) ListarCategorias(ctx context.Context, carpinteriaID int64) ([]stock.Categoria, error) {
	out := []stock.Categoria{}
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT c.id, c.nombre, coalesce(c.descripcion, ''),
			       (SELECT count(*) FROM productos p WHERE p.categoria_id = c.id AND p.activo)::int
			FROM categorias_producto c
			ORDER BY lower(c.nombre)`)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, pgx.RowToStructByPos[stock.Categoria])
		return err
	})
	return out, err
}

func (r *RepoStock) CrearCategoria(ctx context.Context, carpinteriaID int64, c stock.Categoria) (int64, error) {
	var id int64
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO categorias_producto (carpinteria_id, nombre, descripcion) VALUES ($1, $2, $3)
			RETURNING id`, carpinteriaID, c.Nombre, nulo(c.Descripcion)).Scan(&id)
	})
	return id, errorCategoria(err)
}

func (r *RepoStock) ActualizarCategoria(ctx context.Context, carpinteriaID int64, c stock.Categoria) error {
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE categorias_producto SET nombre = $2, descripcion = $3 WHERE id = $1`,
			c.ID, c.Nombre, nulo(c.Descripcion))
		if err == nil && tag.RowsAffected() == 0 {
			return stock.ErrNoEncontrado
		}
		return err
	})
	return errorCategoria(err)
}

func (r *RepoStock) BorrarCategoria(ctx context.Context, carpinteriaID, id int64) error {
	return EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		// La clave foránea deja sin categoría a los productos que la tenían.
		tag, err := tx.Exec(ctx, `DELETE FROM categorias_producto WHERE id = $1`, id)
		if err == nil && tag.RowsAffected() == 0 {
			return stock.ErrNoEncontrado
		}
		return err
	})
}
