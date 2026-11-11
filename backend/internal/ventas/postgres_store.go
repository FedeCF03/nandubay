package ventas

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FedeCF03/nandubay/backend/internal/cortes"
	"github.com/FedeCF03/nandubay/backend/internal/db"
	"github.com/FedeCF03/nandubay/backend/internal/muebles"
	"github.com/FedeCF03/nandubay/backend/internal/stock"
)

// PostgresStore guarda clientes, ventas, pagos, retiros y presupuestos de muebles.
type PostgresStore struct {
	DB *pgxpool.Pool
}

func ventaNoEncontrada(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNoEncontrado
	}
	return err
}

// ---------------------------------------------------------------------------
// Clientes
// ---------------------------------------------------------------------------

func (r *PostgresStore) ListarClientes(ctx context.Context, carpinteriaID int64, buscar string, limite int) ([]Cliente, error) {
	out := []Cliente{}
	err := db.EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, nombre, coalesce(documento,''), coalesce(telefono,''), coalesce(email,''),
			       coalesce(direccion,''), coalesce(notas,'')
			FROM clientes
			WHERE $1 = '' OR nombre ILIKE '%' || $1 || '%' OR documento ILIKE $1 || '%' OR telefono ILIKE '%' || $1 || '%'
			ORDER BY lower(nombre) LIMIT $2`, strings.TrimSpace(buscar), limite)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (Cliente, error) {
			var c Cliente
			err := row.Scan(&c.ID, &c.Nombre, &c.Documento, &c.Telefono, &c.Email, &c.Direccion, &c.Notas)
			return c, err
		})
		return err
	})
	return out, err
}

func (r *PostgresStore) CrearCliente(ctx context.Context, carpinteriaID int64, c Cliente) (int64, error) {
	var id int64
	err := db.EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO clientes (carpinteria_id, nombre, documento, telefono, email, direccion, notas)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
			carpinteriaID, c.Nombre, db.Nulo(c.Documento), db.Nulo(c.Telefono), db.Nulo(c.Email), db.Nulo(c.Direccion), db.Nulo(c.Notas)).Scan(&id)
	})
	return id, err
}

func (r *PostgresStore) ActualizarCliente(ctx context.Context, carpinteriaID int64, c Cliente) error {
	return db.EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE clientes SET nombre = $2, documento = $3, telefono = $4, email = $5, direccion = $6, notas = $7
			WHERE id = $1`,
			c.ID, c.Nombre, db.Nulo(c.Documento), db.Nulo(c.Telefono), db.Nulo(c.Email), db.Nulo(c.Direccion), db.Nulo(c.Notas))
		if err == nil && tag.RowsAffected() == 0 {
			return ErrNoEncontrado
		}
		return err
	})
}

// BuscarProductos y RetazosDisponibles los resuelven los stores de stock y de
// cortes, dueños de esas tablas.
func (r *PostgresStore) BuscarProductos(ctx context.Context, carpinteriaID int64, ids []int64) (map[int64]stock.Producto, error) {
	return (&stock.PostgresStore{DB: r.DB}).BuscarProductos(ctx, carpinteriaID, ids)
}

func (r *PostgresStore) RetazosDisponibles(ctx context.Context, carpinteriaID int64, placaIDs []int64, depositoID int64) (map[int64][]cortes.Retazo, error) {
	return (&cortes.PostgresStore{DB: r.DB}).RetazosDisponibles(ctx, carpinteriaID, placaIDs, depositoID)
}

// ---------------------------------------------------------------------------
// Ventas
// ---------------------------------------------------------------------------

const sqlPagado = `coalesce((SELECT sum(CASE WHEN g.tipo = 'devolucion' THEN -g.monto ELSE g.monto END)
	FROM pagos g WHERE g.pedido_id = p.id), 0)`

func (r *PostgresStore) ListarVentas(ctx context.Context, carpinteriaID int64, f FiltroVentas) ([]ResumenVenta, error) {
	out := []ResumenVenta{}
	buscar := strings.TrimSpace(f.Buscar)
	numero, _ := strconv.Atoi(strings.TrimLeft(buscar, "#0"))
	err := db.EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT p.id, p.numero, p.estado, c.nombre, s.nombre, coalesce(sr.nombre, ''),
			       p.total::float8, (p.total - `+sqlPagado+`)::float8, p.creado_en
			FROM pedidos p
			JOIN clientes c ON c.id = p.cliente_id
			JOIN sucursales s ON s.id = p.sucursal_id
			LEFT JOIN depositos d ON d.id = p.deposito_id
			LEFT JOIN sucursales sr ON sr.id = d.sucursal_id
			WHERE ($5 = '' OR p.tipo = $5)
			  AND ($1 = '' OR p.estado::text = $1
			       OR ($1 = 'a_retirar' AND p.estado IN ('confirmado','en_corte','en_armado','listo')))
			  AND ($2 = '' OR p.numero = $3 OR c.nombre ILIKE '%' || $2 || '%')
			ORDER BY p.numero DESC LIMIT $4`, f.Estado, buscar, numero, f.Limite, string(f.Tipo))
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (ResumenVenta, error) {
			var v ResumenVenta
			err := row.Scan(&v.ID, &v.Numero, &v.Estado, &v.Cliente, &v.Sucursal, &v.Retira, &v.Total, &v.Saldo, &v.Fecha)
			return v, err
		})
		return err
	})
	return out, err
}

func (r *PostgresStore) BuscarVenta(ctx context.Context, carpinteriaID, id int64) (Pedido, error) {
	var p Pedido
	err := db.EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		var err error
		p, err = cargarVenta(ctx, tx, id)
		return err
	})
	return p, err
}

func cargarVenta(ctx context.Context, tx pgx.Tx, id int64) (Pedido, error) {
	var p Pedido
	var retira Lugar
	var retiraID *int64
	var entrega *time.Time
	err := tx.QueryRow(ctx, `
		SELECT p.id, p.numero, p.tipo, p.estado, coalesce(p.notas,''), p.fecha_entrega, p.creado_en, p.confirmado_en,
		       p.reservado_hasta,
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
		&p.ID, &p.Numero, &p.Tipo, &p.Estado, &p.Notas, &entrega, &p.CreadoEn, &p.ConfirmadoEn,
		&p.ReservadoHasta,
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

	// Lo que salió del stock al confirmar, para devolverlo igual si se cancela.
	rows, err := tx.Query(ctx, `
		SELECT producto_id, -sum(cantidad)::float8 FROM stock_movimientos
		WHERE pedido_id = $1 AND tipo = 'venta'
		GROUP BY producto_id ORDER BY min(id)`, id)
	if err != nil {
		return p, err
	}
	for rows.Next() {
		var x Consumo
		if err := rows.Scan(&x.ProductoID, &x.Cantidad); err != nil {
			rows.Close()
			return p, err
		}
		p.Consumido = append(p.Consumido, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return p, err
	}

	// Renglones: productos y muebles, en el orden en que se cargaron.
	type conOrden struct {
		orden int
		l     Linea
	}
	var lineas []conOrden
	rows, err = tx.Query(ctx, `
		SELECT i.id, i.orden, i.producto_id, pr.tipo, coalesce(i.descripcion, pr.nombre),
		       i.cantidad::float8, i.precio_unitario::float8, i.costo_unitario::float8, i.retirado::float8,
		       coalesce(s.ubicacion, '')
		FROM pedido_items i JOIN productos pr ON pr.id = i.producto_id
		LEFT JOIN stock s ON s.producto_id = i.producto_id AND s.deposito_id = $2
		WHERE i.pedido_id = $1 AND i.pedido_mueble_id IS NULL`, id, p.DepositoID)
	if err != nil {
		return p, err
	}
	for rows.Next() {
		var x conOrden
		var tipo stock.Tipo
		x.l.Tipo = LineaProducto
		if err := rows.Scan(&x.l.ID, &x.orden, &x.l.ProductoID, &tipo, &x.l.Descripcion,
			&x.l.Cantidad, &x.l.PrecioUnitario, &x.l.CostoUnitario, &x.l.Retirado, &x.l.Ubicacion); err != nil {
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
		x.l.Tipo, x.l.Unidad = LineaMueble, "unidad"
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
	p.Pagos, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (Pago, error) {
		var g Pago
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
	p.Retiros, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (Retiro, error) {
		var x Retiro
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
			var it RetiroItem
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
	p.Historial, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (CambioEstado, error) {
		var c CambioEstado
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
		return ErrValidacion{Campo: "cliente_id", Mensaje: "elegí un cliente"}
	}
	return nil
}

// guardarLineas reemplaza las líneas de la venta. En un presupuesto de
// muebles, despiece trae las piezas e insumos de cada mueble (uno por línea).
func guardarLineas(ctx context.Context, tx pgx.Tx, carpinteriaID, pedidoID int64, lineas []Linea, despiece []muebles.Mueble) error {
	if _, err := tx.Exec(ctx, `DELETE FROM pedido_items WHERE pedido_id = $1`, pedidoID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM pedido_muebles WHERE pedido_id = $1`, pedidoID); err != nil {
		return err
	}
	for i, l := range lineas {
		var err error
		switch l.Tipo {
		case LineaProducto:
			_, err = tx.Exec(ctx, `
				INSERT INTO pedido_items (carpinteria_id, pedido_id, producto_id, descripcion, cantidad, precio_unitario, costo_unitario, orden)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				carpinteriaID, pedidoID, l.ProductoID, l.Descripcion, l.Cantidad, l.PrecioUnitario, l.CostoUnitario, i)
		case LineaMueble:
			var muebleID int64
			err = tx.QueryRow(ctx, `
				INSERT INTO pedido_muebles (carpinteria_id, pedido_id, nombre, ancho_mm, alto_mm, profundidad_mm, cantidad, precio_unitario, orden, clave)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, coalesce(nullif($10, '')::uuid, gen_random_uuid())) RETURNING id`,
				carpinteriaID, pedidoID, l.Descripcion, db.Nulo(l.AnchoMM), db.Nulo(l.AltoMM), db.Nulo(l.ProfundidadMM), int(l.Cantidad), l.PrecioUnitario, i,
				claveDe(despiece, i)).Scan(&muebleID)
			if err == nil && i < len(despiece) {
				err = guardarDespiece(ctx, tx, carpinteriaID, pedidoID, muebleID, despiece[i])
			}
		}
		if err != nil {
			return err
		}
	}
	if despiece != nil {
		// Las fotos de los muebles que se sacaron del presupuesto se borran.
		if _, err := tx.Exec(ctx, `
			DELETE FROM mueble_fotos WHERE pedido_id = $1
			AND mueble_clave NOT IN (SELECT clave FROM pedido_muebles WHERE pedido_id = $1)`, pedidoID); err != nil {
			return err
		}
	}
	p := Pedido{Lineas: lineas}
	_, err := tx.Exec(ctx, `UPDATE pedidos SET total = $2, costo_total = $3 WHERE id = $1`, pedidoID, p.Total(), p.Costo())
	return err
}

func claveDe(despiece []muebles.Mueble, i int) string {
	if i < len(despiece) {
		return despiece[i].Clave
	}
	return ""
}

// guardarDespiece guarda las piezas y los insumos de un mueble. Los productos
// ya se buscaron con RLS al validar, así que son de esta carpintería.
func guardarDespiece(ctx context.Context, tx pgx.Tx, carpinteriaID, pedidoID, muebleID int64, m muebles.Mueble) error {
	for _, p := range m.Piezas {
		if _, err := tx.Exec(ctx, `
			INSERT INTO piezas (carpinteria_id, pedido_mueble_id, nombre, placa_id, largo_mm, ancho_mm, cantidad,
			                    respeta_veta, canto_l1, canto_l2, canto_a1, canto_a2, canto_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
			carpinteriaID, muebleID, p.Nombre, p.PlacaID, p.Largo, p.Ancho, p.Cantidad, p.RespetaVeta,
			p.Cantos.L1, p.Cantos.L2, p.Cantos.A1, p.Cantos.A2, db.Nulo(p.CantoID)); err != nil {
			return err
		}
	}
	for i, ins := range m.Insumos {
		if _, err := tx.Exec(ctx, `
			INSERT INTO pedido_items (carpinteria_id, pedido_id, pedido_mueble_id, producto_id, cantidad, costo_unitario, orden)
			VALUES ($1, $2, $3, $4, $5, (SELECT costo FROM productos WHERE id = $4), $6)`,
			carpinteriaID, pedidoID, muebleID, ins.ProductoID, ins.Cantidad, i); err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresStore) BuscarDespiece(ctx context.Context, carpinteriaID, pedidoID int64) ([]muebles.Mueble, error) {
	var out []muebles.Mueble
	err := db.EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, clave::text, nombre, coalesce(ancho_mm,0), coalesce(alto_mm,0), coalesce(profundidad_mm,0), cantidad, precio_unitario::float8
			FROM pedido_muebles WHERE pedido_id = $1 ORDER BY orden, id`, pedidoID)
		if err != nil {
			return err
		}
		var ids []int64
		indice := map[int64]int{}
		for rows.Next() {
			var id int64
			var m muebles.Mueble
			if err := rows.Scan(&id, &m.Clave, &m.Nombre, &m.AnchoMM, &m.AltoMM, &m.ProfundidadMM, &m.Cantidad, &m.Precio); err != nil {
				rows.Close()
				return err
			}
			m.Piezas, m.Insumos = []muebles.Pieza{}, []muebles.Insumo{}
			indice[id] = len(out)
			ids = append(ids, id)
			out = append(out, m)
		}
		rows.Close()
		if err := rows.Err(); err != nil || len(ids) == 0 {
			return err
		}
		rows, err = tx.Query(ctx, `
			SELECT pedido_mueble_id, nombre, placa_id, largo_mm, ancho_mm, cantidad, respeta_veta,
			       canto_l1, canto_l2, canto_a1, canto_a2, coalesce(canto_id, 0)
			FROM piezas WHERE pedido_mueble_id = ANY($1) ORDER BY id`, ids)
		if err != nil {
			return err
		}
		for rows.Next() {
			var mid int64
			var p muebles.Pieza
			if err := rows.Scan(&mid, &p.Nombre, &p.PlacaID, &p.Largo, &p.Ancho, &p.Cantidad, &p.RespetaVeta,
				&p.Cantos.L1, &p.Cantos.L2, &p.Cantos.A1, &p.Cantos.A2, &p.CantoID); err != nil {
				rows.Close()
				return err
			}
			out[indice[mid]].Piezas = append(out[indice[mid]].Piezas, p)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		rows, err = tx.Query(ctx, `
			SELECT pedido_mueble_id, producto_id, cantidad::float8
			FROM pedido_items WHERE pedido_mueble_id = ANY($1) ORDER BY orden, id`, ids)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var mid int64
			var ins muebles.Insumo
			if err := rows.Scan(&mid, &ins.ProductoID, &ins.Cantidad); err != nil {
				return err
			}
			out[indice[mid]].Insumos = append(out[indice[mid]].Insumos, ins)
		}
		return rows.Err()
	})
	if out == nil {
		out = []muebles.Mueble{}
	}
	return out, err
}

func (r *PostgresStore) CrearVenta(ctx context.Context, carpinteriaID int64, v VentaNueva) (int64, error) {
	var id int64
	err := db.EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		if err := verificarCliente(ctx, tx, v.Datos.ClienteID); err != nil {
			return err
		}
		// El número es correlativo por carpintería: se toma con un candado
		// para que dos ventas a la vez no saquen el mismo.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(4201, $1::int)`, carpinteriaID); err != nil {
			return err
		}
		err := tx.QueryRow(ctx, `
			INSERT INTO pedidos (carpinteria_id, numero, sucursal_id, cliente_id, vendedor_id, notas, fecha_entrega, token_publico, tipo)
			VALUES ($1, (SELECT coalesce(max(numero), 0) + 1 FROM pedidos), $2, $3, $4, $5, $6::date, gen_random_uuid()::text, $7)
			RETURNING id`,
			carpinteriaID, v.SucursalID, v.Datos.ClienteID, v.VendedorID, db.Nulo(strings.TrimSpace(v.Datos.Notas)), v.Datos.FechaEntrega, string(v.Tipo)).Scan(&id)
		if err != nil {
			return err
		}
		if err := guardarLineas(ctx, tx, carpinteriaID, id, v.Lineas, v.Despiece); err != nil {
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
		return ErrValidacion{Campo: "fecha_entrega", Mensaje: "la fecha de entrega no es válida"}
	}
	return err
}

func (r *PostgresStore) Modificar(ctx context.Context, carpinteriaID, usuarioID, id int64, fn func(Pedido) (Cambio, error)) error {
	err := db.EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
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
				id, c.Datos.ClienteID, db.Nulo(strings.TrimSpace(c.Datos.Notas)), c.Datos.FechaEntrega); err != nil {
				return err
			}
		}
		if c.Lineas != nil {
			if err := guardarLineas(ctx, tx, carpinteriaID, id, c.Lineas, c.Despiece); err != nil {
				return err
			}
		}
		if c.DepositoID != nil {
			var ok bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM depositos WHERE id = $1 AND activo)`, *c.DepositoID).Scan(&ok); err != nil {
				return err
			}
			if !ok {
				return ErrValidacion{Campo: "deposito_id", Mensaje: "elegí de qué depósito sale la mercadería"}
			}
			if _, err := tx.Exec(ctx, `UPDATE pedidos SET deposito_id = $2, confirmado_en = now() WHERE id = $1`, id, *c.DepositoID); err != nil {
				return err
			}
		}
		deposito := p.DepositoID
		if c.DepositoID != nil {
			deposito = c.DepositoID
		}
		if len(c.Disponible) > 0 && deposito != nil {
			if err := controlarDisponible(ctx, tx, carpinteriaID, id, *deposito, c.Disponible); err != nil {
				return err
			}
		}
		if c.ReservarHasta != nil {
			if _, err := tx.Exec(ctx, `UPDATE pedidos SET reservado_hasta = $2 WHERE id = $1`, id, *c.ReservarHasta); err != nil {
				return err
			}
		}
		if err := stock.AplicarOperaciones(ctx, tx, carpinteriaID, usuarioID, &id, c.Operaciones); err != nil {
			return err
		}
		if c.Materiales != nil && c.DepositoID != nil {
			if err := guardarMateriales(ctx, tx, carpinteriaID, usuarioID, id, *c.DepositoID, c.Materiales); err != nil {
				return err
			}
		}
		if c.Estado != nil && *c.Estado == Cancelado {
			if err := anularCortesPendientes(ctx, tx, id); err != nil {
				return err
			}
		}
		if g := c.Pago; g != nil {
			if _, err := tx.Exec(ctx, `
				INSERT INTO pagos (carpinteria_id, pedido_id, tipo, monto, medio, usuario_id, nota)
				VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				carpinteriaID, id, g.Tipo, g.Monto, g.Medio, usuarioID, db.Nulo(g.Nota)); err != nil {
				return err
			}
		}
		if rt := c.Retiro; rt != nil {
			var retiroID int64
			if err := tx.QueryRow(ctx, `
				INSERT INTO retiros (carpinteria_id, pedido_id, sucursal_id, usuario_id, nota)
				VALUES ($1, $2, $3, $4, $5) RETURNING id`,
				carpinteriaID, id, rt.SucursalID, usuarioID, db.Nulo(rt.Nota)).Scan(&retiroID); err != nil {
				return err
			}
			for _, it := range rt.Items {
				tabla, columna := "pedido_items", "pedido_item_id"
				if it.Tipo == LineaMueble {
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

// controlarDisponible confirma que en el depósito alcanza lo que hay menos
// lo reservado por otras ventas para cada producto. Bloquea los saldos (en
// orden de producto, para no trabarse con otra venta) así dos ventas no
// reservan a la vez lo mismo.
func controlarDisponible(ctx context.Context, tx pgx.Tx, carpinteriaID, pedidoID, depositoID int64, cs []Consumo) error {
	orden := make([]Consumo, len(cs))
	copy(orden, cs)
	sort.Slice(orden, func(i, j int) bool { return orden[i].ProductoID < orden[j].ProductoID })
	for _, x := range orden {
		if _, err := tx.Exec(ctx, `
				INSERT INTO stock (carpinteria_id, producto_id, deposito_id, cantidad) VALUES ($1, $2, $3, 0)
				ON CONFLICT (producto_id, deposito_id) DO NOTHING`, carpinteriaID, x.ProductoID, depositoID); err != nil {
			return err
		}
		var hay float64
		if err := tx.QueryRow(ctx, `
				SELECT cantidad::float8 FROM stock WHERE producto_id = $1 AND deposito_id = $2 FOR UPDATE`,
			x.ProductoID, depositoID).Scan(&hay); err != nil {
			return err
		}
		var reservado float64
		var nombre string
		if err := tx.QueryRow(ctx, `
				SELECT coalesce((SELECT sum(cantidad) FROM v_stock_reservado
				                 WHERE producto_id = $1 AND deposito_id = $2 AND pedido_id <> $3), 0)::float8,
				       (SELECT nombre FROM productos WHERE id = $1)`,
			x.ProductoID, depositoID, pedidoID).Scan(&reservado, &nombre); err != nil {
			return err
		}
		if err := stock.Faltante(nombre, hay, reservado, x.Cantidad); err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresStore) ProductosVendidos(ctx context.Context, carpinteriaID int64, f FiltroVendidos) ([]ProductoVendido, error) {
	out := []ProductoVendido{}
	err := db.EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		// Solo productos sueltos (no los insumos de un mueble) de ventas
		// confirmadas y no canceladas.
		rows, err := tx.Query(ctx, `
			SELECT pr.id, pr.nombre, pr.tipo, sum(i.cantidad)::float8,
			       sum(i.cantidad * i.precio_unitario)::float8, sum(i.cantidad * i.costo_unitario)::float8,
			       count(DISTINCT p.id)::int
			FROM pedido_items i
			JOIN pedidos p ON p.id = i.pedido_id
			JOIN productos pr ON pr.id = i.producto_id
			WHERE i.pedido_mueble_id IS NULL
			  AND p.estado NOT IN ('presupuesto', 'cancelado')
			  AND p.confirmado_en >= $1 AND p.confirmado_en < $2
			  AND ($3 = 0 OR p.sucursal_id = $3)
			GROUP BY pr.id, pr.nombre, pr.tipo
			ORDER BY 5 DESC, pr.nombre`, f.Desde, f.Hasta, f.SucursalID)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (ProductoVendido, error) {
			var v ProductoVendido
			err := row.Scan(&v.ProductoID, &v.Nombre, &v.Tipo, &v.Cantidad, &v.Importe, &v.Costo, &v.Ventas)
			v.Unidad = stock.UnidadDe(stock.Tipo(v.Tipo))
			return v, err
		})
		return err
	})
	return out, err
}

func (r *PostgresStore) BuscarMaterialesConfirmados(ctx context.Context, carpinteriaID, pedidoID int64) (*muebles.Materiales, error) {
	var crudo []byte
	err := db.EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
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
		if _, err := cortes.InsertarCorte(ctx, tx, carpinteriaID, usuarioID, &pedidoID, depositoID, "", p); err != nil {
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
