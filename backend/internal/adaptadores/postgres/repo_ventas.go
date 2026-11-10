package postgres

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FedeCF03/nandubay/backend/internal/aplicacion"
	"github.com/FedeCF03/nandubay/backend/internal/dominio/stock"
	"github.com/FedeCF03/nandubay/backend/internal/dominio/ventas"
)

// RepoVentas implementa aplicacion.RepositorioVentas.
type RepoVentas struct {
	DB *pgxpool.Pool
}

var _ aplicacion.RepositorioVentas = (*RepoVentas)(nil)

func ventaNoEncontrada(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ventas.ErrNoEncontrado
	}
	return err
}

// ---------------------------------------------------------------------------
// Clientes
// ---------------------------------------------------------------------------

func (r *RepoVentas) ListarClientes(ctx context.Context, carpinteriaID int64, buscar string, limite int) ([]ventas.Cliente, error) {
	out := []ventas.Cliente{}
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, nombre, coalesce(documento,''), coalesce(telefono,''), coalesce(email,''),
			       coalesce(direccion,''), coalesce(notas,'')
			FROM clientes
			WHERE $1 = '' OR nombre ILIKE '%' || $1 || '%' OR documento ILIKE $1 || '%' OR telefono ILIKE '%' || $1 || '%'
			ORDER BY lower(nombre) LIMIT $2`, strings.TrimSpace(buscar), limite)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (ventas.Cliente, error) {
			var c ventas.Cliente
			err := row.Scan(&c.ID, &c.Nombre, &c.Documento, &c.Telefono, &c.Email, &c.Direccion, &c.Notas)
			return c, err
		})
		return err
	})
	return out, err
}

func (r *RepoVentas) CrearCliente(ctx context.Context, carpinteriaID int64, c ventas.Cliente) (int64, error) {
	var id int64
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO clientes (carpinteria_id, nombre, documento, telefono, email, direccion, notas)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
			carpinteriaID, c.Nombre, nulo(c.Documento), nulo(c.Telefono), nulo(c.Email), nulo(c.Direccion), nulo(c.Notas)).Scan(&id)
	})
	return id, err
}

func (r *RepoVentas) ActualizarCliente(ctx context.Context, carpinteriaID int64, c ventas.Cliente) error {
	return EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE clientes SET nombre = $2, documento = $3, telefono = $4, email = $5, direccion = $6, notas = $7
			WHERE id = $1`,
			c.ID, c.Nombre, nulo(c.Documento), nulo(c.Telefono), nulo(c.Email), nulo(c.Direccion), nulo(c.Notas))
		if err == nil && tag.RowsAffected() == 0 {
			return ventas.ErrNoEncontrado
		}
		return err
	})
}

func (r *RepoVentas) BuscarProductos(ctx context.Context, carpinteriaID int64, ids []int64) (map[int64]stock.Producto, error) {
	out := map[int64]stock.Producto{}
	if len(ids) == 0 {
		return out, nil
	}
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, tipo, nombre, costo::float8, precio_venta::float8
			FROM productos WHERE id = ANY($1) AND activo`, ids)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var p stock.Producto
			if err := rows.Scan(&p.ID, &p.Tipo, &p.Nombre, &p.Costo, &p.PrecioVenta); err != nil {
				return err
			}
			out[p.ID] = p
		}
		return rows.Err()
	})
	return out, err
}

// ---------------------------------------------------------------------------
// Ventas
// ---------------------------------------------------------------------------

const sqlPagado = `coalesce((SELECT sum(CASE WHEN g.tipo = 'devolucion' THEN -g.monto ELSE g.monto END)
	FROM pagos g WHERE g.pedido_id = p.id), 0)`

func (r *RepoVentas) ListarVentas(ctx context.Context, carpinteriaID int64, f aplicacion.FiltroVentas) ([]aplicacion.ResumenVenta, error) {
	out := []aplicacion.ResumenVenta{}
	buscar := strings.TrimSpace(f.Buscar)
	numero, _ := strconv.Atoi(strings.TrimLeft(buscar, "#0"))
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT p.id, p.numero, p.estado, c.nombre, s.nombre, coalesce(sr.nombre, ''),
			       p.total::float8, (p.total - `+sqlPagado+`)::float8, p.creado_en
			FROM pedidos p
			JOIN clientes c ON c.id = p.cliente_id
			JOIN sucursales s ON s.id = p.sucursal_id
			LEFT JOIN depositos d ON d.id = p.deposito_id
			LEFT JOIN sucursales sr ON sr.id = d.sucursal_id
			WHERE ($1 = '' OR p.estado::text = $1
			       OR ($1 = 'a_retirar' AND p.estado IN ('confirmado','en_corte','en_armado','listo')))
			  AND ($2 = '' OR p.numero = $3 OR c.nombre ILIKE '%' || $2 || '%')
			ORDER BY p.numero DESC LIMIT $4`, f.Estado, buscar, numero, f.Limite)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (aplicacion.ResumenVenta, error) {
			var v aplicacion.ResumenVenta
			err := row.Scan(&v.ID, &v.Numero, &v.Estado, &v.Cliente, &v.Sucursal, &v.Retira, &v.Total, &v.Saldo, &v.Fecha)
			return v, err
		})
		return err
	})
	return out, err
}

func (r *RepoVentas) BuscarVenta(ctx context.Context, carpinteriaID, id int64) (ventas.Pedido, error) {
	var p ventas.Pedido
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		var err error
		p, err = cargarVenta(ctx, tx, id)
		return err
	})
	return p, err
}

func cargarVenta(ctx context.Context, tx pgx.Tx, id int64) (ventas.Pedido, error) {
	var p ventas.Pedido
	var retira ventas.Lugar
	var retiraID *int64
	var entrega *time.Time
	err := tx.QueryRow(ctx, `
		SELECT p.id, p.numero, p.estado, coalesce(p.notas,''), p.fecha_entrega, p.creado_en, p.confirmado_en,
		       s.id, s.nombre, coalesce(s.direccion,''), coalesce(s.telefono,''),
		       p.deposito_id, coalesce(d.nombre,''),
		       sr.id, coalesce(sr.nombre,''), coalesce(sr.direccion,''), coalesce(sr.telefono,''),
		       c.id, c.nombre, coalesce(c.documento,''), coalesce(c.telefono,''), coalesce(c.email,''),
		       coalesce(c.direccion,''), coalesce(c.notas,''),
		       coalesce(v.nombre,'')
		FROM pedidos p
		JOIN sucursales s ON s.id = p.sucursal_id
		JOIN clientes c ON c.id = p.cliente_id
		LEFT JOIN usuarios v ON v.id = p.vendedor_id
		LEFT JOIN depositos d ON d.id = p.deposito_id
		LEFT JOIN sucursales sr ON sr.id = d.sucursal_id
		WHERE p.id = $1`, id).Scan(
		&p.ID, &p.Numero, &p.Estado, &p.Notas, &entrega, &p.CreadoEn, &p.ConfirmadoEn,
		&p.Sucursal.ID, &p.Sucursal.Nombre, &p.Sucursal.Direccion, &p.Sucursal.Telefono,
		&p.DepositoID, &p.Deposito,
		&retiraID, &retira.Nombre, &retira.Direccion, &retira.Telefono,
		&p.Cliente.ID, &p.Cliente.Nombre, &p.Cliente.Documento, &p.Cliente.Telefono, &p.Cliente.Email,
		&p.Cliente.Direccion, &p.Cliente.Notas,
		&p.Vendedor)
	if err != nil {
		return p, ventaNoEncontrada(err)
	}
	if retiraID != nil {
		retira.ID = *retiraID
		p.Retira = &retira
	}
	if entrega != nil {
		s := entrega.Format("2006-01-02")
		p.FechaEntrega = &s
	}

	// Renglones: productos y muebles, en el orden en que se cargaron.
	type conOrden struct {
		orden int
		l     ventas.Linea
	}
	var lineas []conOrden
	rows, err := tx.Query(ctx, `
		SELECT i.id, i.orden, i.producto_id, pr.tipo, coalesce(i.descripcion, pr.nombre),
		       i.cantidad::float8, i.precio_unitario::float8, i.costo_unitario::float8, i.retirado::float8
		FROM pedido_items i JOIN productos pr ON pr.id = i.producto_id
		WHERE i.pedido_id = $1 AND i.pedido_mueble_id IS NULL`, id)
	if err != nil {
		return p, err
	}
	for rows.Next() {
		var x conOrden
		var tipo stock.Tipo
		x.l.Tipo = ventas.LineaProducto
		if err := rows.Scan(&x.l.ID, &x.orden, &x.l.ProductoID, &tipo, &x.l.Descripcion,
			&x.l.Cantidad, &x.l.PrecioUnitario, &x.l.CostoUnitario, &x.l.Retirado); err != nil {
			rows.Close()
			return p, err
		}
		x.l.Unidad = stock.UnidadDe(tipo)
		lineas = append(lineas, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return p, err
	}
	rows, err = tx.Query(ctx, `
		SELECT id, orden, nombre, coalesce(ancho_mm,0), coalesce(alto_mm,0), coalesce(profundidad_mm,0),
		       cantidad::float8, precio_unitario::float8, retirado::float8
		FROM pedido_muebles WHERE pedido_id = $1`, id)
	if err != nil {
		return p, err
	}
	for rows.Next() {
		var x conOrden
		x.l.Tipo, x.l.Unidad = ventas.LineaMueble, "unidad"
		if err := rows.Scan(&x.l.ID, &x.orden, &x.l.Descripcion, &x.l.AnchoMM, &x.l.AltoMM, &x.l.ProfundidadMM,
			&x.l.Cantidad, &x.l.PrecioUnitario, &x.l.Retirado); err != nil {
			rows.Close()
			return p, err
		}
		lineas = append(lineas, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return p, err
	}
	sort.SliceStable(lineas, func(i, j int) bool { return lineas[i].orden < lineas[j].orden })
	for _, x := range lineas {
		p.Lineas = append(p.Lineas, x.l)
	}

	rows, err = tx.Query(ctx, `
		SELECT g.id, g.tipo, g.monto::float8, g.medio, coalesce(g.nota,''), coalesce(u.nombre,''), g.creado_en
		FROM pagos g LEFT JOIN usuarios u ON u.id = g.usuario_id
		WHERE g.pedido_id = $1 ORDER BY g.creado_en, g.id`, id)
	if err != nil {
		return p, err
	}
	p.Pagos, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (ventas.Pago, error) {
		var g ventas.Pago
		err := row.Scan(&g.ID, &g.Tipo, &g.Monto, &g.Medio, &g.Nota, &g.Usuario, &g.Fecha)
		return g, err
	})
	if err != nil {
		return p, err
	}

	rows, err = tx.Query(ctx, `
		SELECT r.id, s.nombre, coalesce(u.nombre,''), coalesce(r.nota,''), r.creado_en
		FROM retiros r JOIN sucursales s ON s.id = r.sucursal_id LEFT JOIN usuarios u ON u.id = r.usuario_id
		WHERE r.pedido_id = $1 ORDER BY r.creado_en, r.id`, id)
	if err != nil {
		return p, err
	}
	p.Retiros, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (ventas.Retiro, error) {
		var x ventas.Retiro
		err := row.Scan(&x.ID, &x.Sucursal, &x.Usuario, &x.Nota, &x.Fecha)
		return x, err
	})
	if err != nil {
		return p, err
	}
	if len(p.Retiros) > 0 {
		porRetiro := map[int64]int{}
		for i, x := range p.Retiros {
			porRetiro[x.ID] = i
		}
		rows, err = tx.Query(ctx, `
			SELECT ri.retiro_id, coalesce(ri.pedido_item_id, ri.pedido_mueble_id),
			       CASE WHEN ri.pedido_item_id IS NULL THEN 'mueble' ELSE 'producto' END,
			       coalesce(i.descripcion, pr.nombre, m.nombre), ri.cantidad::float8
			FROM retiro_items ri
			JOIN retiros r ON r.id = ri.retiro_id
			LEFT JOIN pedido_items i ON i.id = ri.pedido_item_id
			LEFT JOIN productos pr ON pr.id = i.producto_id
			LEFT JOIN pedido_muebles m ON m.id = ri.pedido_mueble_id
			WHERE r.pedido_id = $1 ORDER BY ri.id`, id)
		if err != nil {
			return p, err
		}
		for rows.Next() {
			var rid int64
			var it ventas.RetiroItem
			if err := rows.Scan(&rid, &it.LineaID, &it.Tipo, &it.Descripcion, &it.Cantidad); err != nil {
				rows.Close()
				return p, err
			}
			i := porRetiro[rid]
			p.Retiros[i].Items = append(p.Retiros[i].Items, it)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return p, err
		}
	}

	rows, err = tx.Query(ctx, `
		SELECT e.estado, coalesce(u.nombre,''), e.creado_en
		FROM pedido_estados e LEFT JOIN usuarios u ON u.id = e.usuario_id
		WHERE e.pedido_id = $1 ORDER BY e.creado_en, e.id`, id)
	if err != nil {
		return p, err
	}
	p.Historial, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (ventas.CambioEstado, error) {
		var c ventas.CambioEstado
		err := row.Scan(&c.Estado, &c.Usuario, &c.Fecha)
		return c, err
	})
	return p, err
}

// verificarCliente confirma que el cliente es de la carpintería: las claves
// foráneas no pasan por RLS.
func verificarCliente(ctx context.Context, tx pgx.Tx, id int64) error {
	var ok bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM clientes WHERE id = $1)`, id).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return ventas.ErrValidacion{Campo: "cliente_id", Mensaje: "elegí un cliente"}
	}
	return nil
}

func guardarLineas(ctx context.Context, tx pgx.Tx, carpinteriaID, pedidoID int64, lineas []ventas.Linea) error {
	if _, err := tx.Exec(ctx, `DELETE FROM pedido_items WHERE pedido_id = $1`, pedidoID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM pedido_muebles WHERE pedido_id = $1`, pedidoID); err != nil {
		return err
	}
	for i, l := range lineas {
		var err error
		switch l.Tipo {
		case ventas.LineaProducto:
			_, err = tx.Exec(ctx, `
				INSERT INTO pedido_items (carpinteria_id, pedido_id, producto_id, descripcion, cantidad, precio_unitario, costo_unitario, orden)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				carpinteriaID, pedidoID, l.ProductoID, l.Descripcion, l.Cantidad, l.PrecioUnitario, l.CostoUnitario, i)
		case ventas.LineaMueble:
			_, err = tx.Exec(ctx, `
				INSERT INTO pedido_muebles (carpinteria_id, pedido_id, nombre, ancho_mm, alto_mm, profundidad_mm, cantidad, precio_unitario, orden)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
				carpinteriaID, pedidoID, l.Descripcion, nulo(l.AnchoMM), nulo(l.AltoMM), nulo(l.ProfundidadMM), int(l.Cantidad), l.PrecioUnitario, i)
		}
		if err != nil {
			return err
		}
	}
	p := ventas.Pedido{Lineas: lineas}
	_, err := tx.Exec(ctx, `UPDATE pedidos SET total = $2, costo_total = $3 WHERE id = $1`, pedidoID, p.Total(), p.Costo())
	return err
}

func (r *RepoVentas) CrearVenta(ctx context.Context, carpinteriaID int64, v aplicacion.VentaNueva) (int64, error) {
	var id int64
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		if err := verificarCliente(ctx, tx, v.Datos.ClienteID); err != nil {
			return err
		}
		// El número es correlativo por carpintería: se toma con un candado
		// para que dos ventas a la vez no saquen el mismo.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(4201, $1::int)`, carpinteriaID); err != nil {
			return err
		}
		err := tx.QueryRow(ctx, `
			INSERT INTO pedidos (carpinteria_id, numero, sucursal_id, cliente_id, vendedor_id, notas, fecha_entrega, token_publico)
			VALUES ($1, (SELECT coalesce(max(numero), 0) + 1 FROM pedidos), $2, $3, $4, $5, $6::date, gen_random_uuid()::text)
			RETURNING id`,
			carpinteriaID, v.SucursalID, v.Datos.ClienteID, v.VendedorID, nulo(strings.TrimSpace(v.Datos.Notas)), v.Datos.FechaEntrega).Scan(&id)
		if err != nil {
			return err
		}
		if err := guardarLineas(ctx, tx, carpinteriaID, id, v.Lineas); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO pedido_estados (carpinteria_id, pedido_id, estado, usuario_id) VALUES ($1, $2, 'presupuesto', $3)`,
			carpinteriaID, id, v.VendedorID)
		return err
	})
	return id, traducirFecha(err)
}

// traducirFecha convierte el error de Postgres por una fecha mal escrita.
func traducirFecha(err error) error {
	if err != nil && strings.Contains(err.Error(), "invalid input syntax for type date") {
		return ventas.ErrValidacion{Campo: "fecha_entrega", Mensaje: "la fecha de entrega no es válida"}
	}
	return err
}

func (r *RepoVentas) Modificar(ctx context.Context, carpinteriaID, usuarioID, id int64, fn func(ventas.Pedido) (ventas.Cambio, error)) error {
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		// Bloquear la venta: dos retiros o pagos a la vez se hacen uno detrás del otro.
		if err := tx.QueryRow(ctx, `SELECT id FROM pedidos WHERE id = $1 FOR UPDATE`, id).Scan(&id); err != nil {
			return ventaNoEncontrada(err)
		}
		p, err := cargarVenta(ctx, tx, id)
		if err != nil {
			return err
		}
		c, err := fn(p)
		if err != nil {
			return err
		}

		if c.Datos != nil {
			if err := verificarCliente(ctx, tx, c.Datos.ClienteID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE pedidos SET cliente_id = $2, notas = $3, fecha_entrega = $4::date WHERE id = $1`,
				id, c.Datos.ClienteID, nulo(strings.TrimSpace(c.Datos.Notas)), c.Datos.FechaEntrega); err != nil {
				return err
			}
		}
		if c.Lineas != nil {
			if err := guardarLineas(ctx, tx, carpinteriaID, id, c.Lineas); err != nil {
				return err
			}
		}
		if c.DepositoID != nil {
			var ok bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM depositos WHERE id = $1 AND activo)`, *c.DepositoID).Scan(&ok); err != nil {
				return err
			}
			if !ok {
				return ventas.ErrValidacion{Campo: "deposito_id", Mensaje: "elegí de qué depósito sale la mercadería"}
			}
			if _, err := tx.Exec(ctx, `UPDATE pedidos SET deposito_id = $2, confirmado_en = now() WHERE id = $1`, id, *c.DepositoID); err != nil {
				return err
			}
		}
		if err := aplicarOperaciones(ctx, tx, carpinteriaID, usuarioID, &id, c.Operaciones); err != nil {
			return err
		}
		if g := c.Pago; g != nil {
			if _, err := tx.Exec(ctx, `
				INSERT INTO pagos (carpinteria_id, pedido_id, tipo, monto, medio, usuario_id, nota)
				VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				carpinteriaID, id, g.Tipo, g.Monto, g.Medio, usuarioID, nulo(g.Nota)); err != nil {
				return err
			}
		}
		if rt := c.Retiro; rt != nil {
			var retiroID int64
			if err := tx.QueryRow(ctx, `
				INSERT INTO retiros (carpinteria_id, pedido_id, sucursal_id, usuario_id, nota)
				VALUES ($1, $2, $3, $4, $5) RETURNING id`,
				carpinteriaID, id, rt.SucursalID, usuarioID, nulo(rt.Nota)).Scan(&retiroID); err != nil {
				return err
			}
			for _, it := range rt.Items {
				tabla, columna := "pedido_items", "pedido_item_id"
				if it.Tipo == ventas.LineaMueble {
					tabla, columna = "pedido_muebles", "pedido_mueble_id"
				}
				if _, err := tx.Exec(ctx, `UPDATE `+tabla+` SET retirado = retirado + $3 WHERE id = $1 AND pedido_id = $2`,
					it.LineaID, id, it.Cantidad); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `INSERT INTO retiro_items (carpinteria_id, retiro_id, `+columna+`, cantidad) VALUES ($1, $2, $3, $4)`,
					carpinteriaID, retiroID, it.LineaID, it.Cantidad); err != nil {
					return err
				}
			}
		}
		if c.Estado != nil {
			if _, err := tx.Exec(ctx, `
				UPDATE pedidos SET estado = $2::estado_pedido,
				  entregado_en = CASE WHEN $2::estado_pedido = 'entregado' THEN now() ELSE entregado_en END
				WHERE id = $1`, id, *c.Estado); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO pedido_estados (carpinteria_id, pedido_id, estado, usuario_id) VALUES ($1, $2, $3, $4)`,
				carpinteriaID, id, *c.Estado, usuarioID); err != nil {
				return err
			}
		}
		return nil
	})
	return traducirFecha(err)
}
