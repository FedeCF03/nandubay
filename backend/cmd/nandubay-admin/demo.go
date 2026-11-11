package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/FedeCF03/nandubay/backend/internal/modelos/acceso"
	"github.com/FedeCF03/nandubay/backend/internal/modelos/cortes"
	"github.com/FedeCF03/nandubay/backend/internal/modelos/muebles"
	"github.com/FedeCF03/nandubay/backend/internal/modelos/stock"
	"github.com/FedeCF03/nandubay/backend/internal/modelos/ventas"
	"github.com/FedeCF03/nandubay/backend/internal/repositorios"
	"github.com/FedeCF03/nandubay/backend/internal/servicios"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// cargarDemo carga datos de prueba en la carpintería del usuario dado:
// productos, stock en dos depósitos, retazos, clientes, ventas de mostrador,
// presupuestos de muebles en distintos estados y cortes optimizados. Usa los
// mismos casos de uso que la app, así los datos quedan coherentes.
func cargarDemo(args []string) error {
	fs := flag.NewFlagSet("cargar-demo", flag.ExitOnError)
	email := fs.String("email", "", "email de un usuario de la carpintería donde cargar los datos")
	_ = fs.Parse(args)
	if *email == "" {
		return errors.New("falta -email")
	}
	ctx := context.Background()
	pool, err := repositorios.Conectar(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := repositorios.Migrar(pool); err != nil {
		return fmt.Errorf("migraciones: %w", err)
	}
	u, err := usuarioDe(ctx, pool, *email)
	if err != nil {
		return err
	}
	d := &demo{
		ctx: ctx, u: u, pool: pool,
		stock:  &servicios.ServicioStock{Repo: &repositorios.RepoStock{DB: pool}},
		ventas: &servicios.ServicioVentas{Repo: &repositorios.RepoVentas{DB: pool}},
		cortes: servicios.ServicioCortes{Repo: &repositorios.RepoCortes{DB: pool}},
		prod:   map[string]int64{},
	}
	if err := d.cargar(); err != nil {
		return err
	}
	fmt.Printf("Datos de prueba cargados en %s.\n", u.Carpinteria)
	return nil
}

func usuarioDe(ctx context.Context, pool *pgxpool.Pool, email string) (*acceso.Usuario, error) {
	repo := &repositorios.RepoAcceso{DB: pool}
	c, err := repo.BuscarCuenta(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return nil, fmt.Errorf("no encontré el usuario %s: %w", email, err)
	}
	da, err := repo.CargarAcceso(ctx, c.CarpinteriaID, c.UsuarioID)
	if err != nil {
		return nil, err
	}
	sucs, elegida, permisos, err := acceso.ResolverAcceso(da.SucursalesActivas, da.Asignaciones, nil)
	if err != nil {
		return nil, fmt.Errorf("el usuario no tiene sucursales: %w", err)
	}
	return &acceso.Usuario{ID: c.UsuarioID, Nombre: da.Nombre, Email: da.Email, CarpinteriaID: c.CarpinteriaID,
		Carpinteria: da.Carpinteria, SucursalID: elegida, Sucursales: sucs, Permisos: permisos}, nil
}

type demo struct {
	ctx    context.Context
	u      *acceso.Usuario
	pool   *pgxpool.Pool
	stock  *servicios.ServicioStock
	ventas *servicios.ServicioVentas
	cortes servicios.ServicioCortes
	prod   map[string]int64 // sku → id
	dep1   int64            // depósito principal
	dep2   int64            // depósito de la otra sucursal
}

func (d *demo) cargar() error {
	ps, err := d.stock.ListarProductos(d.ctx, d.u, stock.FiltroProductos{Buscar: "DEMO-", Inactivos: true})
	if err != nil {
		return err
	}
	if len(ps) > 0 {
		return errors.New("esta carpintería ya tiene los datos de prueba (hay productos DEMO-)")
	}
	pasos := []struct {
		nombre string
		fn     func() error
	}{
		{"depósitos", d.depositos},
		{"productos y stock", d.productos},
		{"retazos", d.retazos},
		{"ventas", d.ventasDemo},
	}
	for _, p := range pasos {
		fmt.Printf("· %s\n", p.nombre)
		if err := p.fn(); err != nil {
			return fmt.Errorf("%s: %w", p.nombre, err)
		}
	}
	return nil
}

// depositos usa el primer depósito y, si hay uno solo, crea la sucursal
// "La Plata" con su depósito para probar traspasos y retiros en otra sucursal.
func (d *demo) depositos() error {
	ds, err := d.stock.ListarDepositos(d.ctx, d.u)
	if err != nil {
		return err
	}
	if len(ds) == 0 {
		return errors.New("la carpintería no tiene depósitos")
	}
	d.dep1 = ds[0].ID
	if len(ds) > 1 {
		d.dep2 = ds[1].ID
		return nil
	}
	return repositorios.EnCarpinteria(d.ctx, d.pool, d.u.CarpinteriaID, func(tx pgx.Tx) error {
		var suc int64
		if err := tx.QueryRow(d.ctx, `
			INSERT INTO sucursales (carpinteria_id, nombre, direccion, telefono)
			VALUES ($1, 'La Plata', 'Calle 7 N° 1234, La Plata', '221 400 0000')
			ON CONFLICT (carpinteria_id, nombre) DO UPDATE SET activa = true RETURNING id`, d.u.CarpinteriaID).Scan(&suc); err != nil {
			return err
		}
		return tx.QueryRow(d.ctx, `
			INSERT INTO depositos (carpinteria_id, sucursal_id, nombre) VALUES ($1, $2, 'Depósito La Plata')
			ON CONFLICT (carpinteria_id, nombre) DO UPDATE SET activo = true RETURNING id`, d.u.CarpinteriaID, suc).Scan(&d.dep2)
	})
}

type productoDemo struct {
	p          stock.Producto
	dep1, dep2 float64
}

func placa(sku, nombre, color string, costo float64, veta bool, minimo float64, dep1, dep2 float64) productoDemo {
	return productoDemo{stock.Producto{Tipo: stock.TipoPlaca, Sku: sku, Nombre: nombre, Marca: "Faplac", Color: color,
		Costo: costo, PrecioVenta: costo * 1.45, StockMinimo: minimo, LargoMM: 2600, AnchoMM: 1830, EspesorMM: 18,
		TieneVeta: veta, RefiladoMM: 5}, dep1, dep2}
}

func canto(sku, nombre, color string, costo float64, dep1, dep2 float64) productoDemo {
	return productoDemo{stock.Producto{Tipo: stock.TipoCanto, Sku: sku, Nombre: nombre, Color: color, Unidad: "m",
		Costo: costo, PrecioVenta: costo * 1.6, StockMinimo: 30, CantoAnchoMM: 22, CantoEspMM: 0.45}, dep1, dep2}
}

func herraje(sku, nombre, marca string, costo, minimo float64, dep1, dep2 float64) productoDemo {
	return productoDemo{stock.Producto{Tipo: stock.TipoHerraje, Sku: sku, Nombre: nombre, Marca: marca,
		Costo: costo, PrecioVenta: costo * 1.6, StockMinimo: minimo}, dep1, dep2}
}

func (d *demo) productos() error {
	lista := []productoDemo{
		placa("DEMO-BL18", "Melamina Blanca 18 mm", "Blanco", 52000, false, 5, 14, 4),
		placa("DEMO-NE18", "Melamina Negra 18 mm", "Negro", 56000, false, 3, 6, 2),
		placa("DEMO-RO18", "Melamina Roble Dakar 18 mm", "Roble", 68000, true, 3, 5, 2),
		// Grafito queda por debajo del mínimo: aparece en las alertas de stock.
		placa("DEMO-GR18", "Melamina Gris Grafito 18 mm", "Grafito", 61000, false, 4, 2, 0),
		{stock.Producto{Tipo: stock.TipoPlaca, Sku: "DEMO-FIB3", Nombre: "Fibrofácil blanco 3 mm (fondos)", Color: "Blanco",
			Costo: 17000, PrecioVenta: 25000, StockMinimo: 4, LargoMM: 2600, AnchoMM: 1830, EspesorMM: 3}, 8, 3},
		canto("DEMO-CBL", "Canto PVC blanco 22 mm", "Blanco", 350, 180, 60),
		canto("DEMO-CNE", "Canto PVC negro 22 mm", "Negro", 380, 90, 0),
		canto("DEMO-CRO", "Canto PVC roble 22 mm", "Roble", 420, 70, 25),
		canto("DEMO-CGR", "Canto PVC grafito 22 mm", "Grafito", 390, 12, 0),
		herraje("DEMO-BIS35", "Bisagra cazoleta 35 mm cierre suave", "Häfele", 950, 20, 60, 30),
		herraje("DEMO-COR45", "Corredera telescópica 45 cm (par)", "Häfele", 6800, 6, 16, 4),
		herraje("DEMO-PATA", "Pata regulable plástica 10 cm", "", 1200, 12, 40, 12),
		// Manijas bajo el mínimo: otra alerta.
		herraje("DEMO-MAN128", "Manija barral 128 mm acero", "", 2600, 10, 6, 0),
		herraje("DEMO-SOP", "Soporte de estante", "", 120, 50, 200, 80),
		herraje("DEMO-RIEL", "Juego de rieles placard 2 puertas", "", 38000, 1, 3, 1),
		{stock.Producto{Tipo: stock.TipoOtro, Sku: "DEMO-TOR", Nombre: "Tornillo 4 × 50 (caja × 100)", Costo: 3500,
			PrecioVenta: 5500, StockMinimo: 5}, 25, 10},
		{stock.Producto{Tipo: stock.TipoOtro, Sku: "DEMO-COLA", Nombre: "Cola vinílica 1 kg", Costo: 4200,
			PrecioVenta: 6900, StockMinimo: 3}, 9, 2},
	}
	// Dónde se guarda cada tipo de producto en los depósitos.
	lugar := map[stock.Tipo]string{stock.TipoPlaca: "Rack de placas", stock.TipoCanto: "Estante de cantos",
		stock.TipoHerraje: "Cajonera de herrajes", stock.TipoOtro: "Estantería"}
	for i, x := range lista {
		p, err := d.stock.CrearProducto(d.ctx, d.u, x.p)
		if err != nil {
			return fmt.Errorf("%s: %w", x.p.Nombre, err)
		}
		d.prod[x.p.Sku] = p.ID
		costo := x.p.Costo
		for dep, cant := range map[int64]float64{d.dep1: x.dep1, d.dep2: x.dep2} {
			if cant > 0 {
				if err := d.stock.Ingresar(d.ctx, d.u, p.ID, dep, cant, &costo, "Carga inicial (datos de prueba)"); err != nil {
					return fmt.Errorf("%s: %w", x.p.Nombre, err)
				}
			}
		}
		ub := fmt.Sprintf("%s %d", lugar[x.p.Tipo], i%4+1)
		if err := d.stock.GuardarUbicaciones(d.ctx, d.u, p.ID, []stock.UbicacionNueva{
			{DepositoID: d.dep1, Ubicacion: ub}, {DepositoID: d.dep2, Ubicacion: ub},
		}); err != nil {
			return fmt.Errorf("%s: %w", x.p.Nombre, err)
		}
	}
	return nil
}

func (d *demo) retazos() error {
	lista := []stock.RetazoStock{
		{PlacaID: d.prod["DEMO-BL18"], DepositoID: d.dep1, Largo: 1200, Ancho: 600, Nota: "Sobrante de un placard"},
		{PlacaID: d.prod["DEMO-BL18"], DepositoID: d.dep1, Largo: 900, Ancho: 450},
		{PlacaID: d.prod["DEMO-BL18"], DepositoID: d.dep1, Largo: 760, Ancho: 320},
		{PlacaID: d.prod["DEMO-RO18"], DepositoID: d.dep1, Largo: 1500, Ancho: 800, Nota: "Veta a lo largo"},
		{PlacaID: d.prod["DEMO-NE18"], DepositoID: d.dep2, Largo: 1000, Ancho: 400},
	}
	for _, r := range lista {
		if _, err := d.stock.CrearRetazo(d.ctx, d.u, r); err != nil {
			return err
		}
	}
	return nil
}

func pz(nombre string, placa int64, largo, ancho, cant int, cantos cortes.Cantos, canto int64, vertical bool) muebles.Pieza {
	p := muebles.Pieza{Nombre: nombre, PlacaID: placa, Largo: largo, Ancho: ancho, Cantidad: cant, Cantos: cantos, RespetaVeta: true, VetaVertical: vertical}
	if cantos.L1 || cantos.L2 || cantos.A1 || cantos.A2 {
		p.CantoID = canto
	}
	return p
}

var (
	sinCanto = cortes.Cantos{}
	unLargo  = cortes.Cantos{L1: true}
	todos    = cortes.Cantos{L1: true, L2: true, A1: true, A2: true}
)

func fecha(dias int) *string {
	s := time.Now().AddDate(0, 0, dias).Format("2006-01-02")
	return &s
}

func (d *demo) ventasDemo() error {
	clientes := map[string]int64{}
	for _, c := range []ventas.Cliente{
		{Nombre: "Ana Ruiz", Telefono: "221 555 0101", Direccion: "Calle 12 N° 845, La Plata"},
		{Nombre: "Martín Gómez", Telefono: "221 555 0202", Email: "martin.gomez@ejemplo.com", Direccion: "Camino Belgrano 4500, Gonnet"},
		{Nombre: "Lucía Fernández", Telefono: "221 555 0303", Documento: "30.123.456"},
		{Nombre: "Estudio Arquitectura Norte", Telefono: "221 555 0404", Email: "obras@ejemplo.com", Notas: "Pide factura por mail"},
		{Nombre: "Consumidor final"},
	} {
		x, err := d.ventas.CrearCliente(d.ctx, d.u, c)
		if err != nil {
			return err
		}
		clientes[c.Nombre] = x.ID
	}
	p := d.prod

	// Ventas de mostrador: una paga y entregada, una confirmada con seña y un presupuesto.
	lineas := func(xs ...any) []ventas.LineaNueva {
		var out []ventas.LineaNueva
		for i := 0; i < len(xs); i += 2 {
			out = append(out, ventas.LineaNueva{Tipo: ventas.LineaProducto, ProductoID: p[xs[i].(string)], Cantidad: xs[i+1].(float64)})
		}
		return out
	}
	v1, err := d.ventas.CrearPresupuesto(d.ctx, d.u, ventas.Mostrador, ventas.DatosVenta{ClienteID: clientes["Consumidor final"]},
		lineas("DEMO-BIS35", 8.0, "DEMO-CBL", 10.0, "DEMO-TOR", 1.0))
	if err != nil {
		return err
	}
	if _, err := d.ventas.Confirmar(d.ctx, d.u, v1.ID, d.dep1); err != nil {
		return err
	}
	pagada, err := d.ventas.RegistrarPago(d.ctx, d.u, v1.ID, ventas.Pago{Tipo: ventas.PagoPago, Monto: v1.Total, Medio: "efectivo"})
	if err != nil {
		return err
	}
	// Se lleva todo: recién ahí sale del stock.
	var items []ventas.RetiroItem
	for _, l := range pagada.Lineas {
		items = append(items, ventas.RetiroItem{LineaID: l.ID, Tipo: l.Tipo, Cantidad: l.Cantidad})
	}
	if _, err := d.ventas.Retirar(d.ctx, d.u, v1.ID, items, "Se lo llevó en el momento"); err != nil {
		return err
	}
	v2, err := d.ventas.CrearPresupuesto(d.ctx, d.u, ventas.Mostrador, ventas.DatosVenta{ClienteID: clientes["Estudio Arquitectura Norte"]},
		lineas("DEMO-BL18", 3.0, "DEMO-CBL", 25.0, "DEMO-COR45", 4.0))
	if err != nil {
		return err
	}
	if _, err := d.ventas.Confirmar(d.ctx, d.u, v2.ID, d.dep1); err != nil {
		return err
	}
	if _, err := d.ventas.RegistrarPago(d.ctx, d.u, v2.ID, ventas.Pago{Tipo: ventas.PagoSena, Monto: 100000, Medio: "transferencia"}); err != nil {
		return err
	}
	if _, err := d.ventas.CrearPresupuesto(d.ctx, d.u, ventas.Mostrador, ventas.DatosVenta{ClienteID: clientes["Lucía Fernández"], Notas: "Consultó por color grafito"},
		lineas("DEMO-GR18", 2.0, "DEMO-CGR", 15.0)); err != nil {
		return err
	}

	// Presupuestos de muebles.
	bl, ne, ro, fib := p["DEMO-BL18"], p["DEMO-NE18"], p["DEMO-RO18"], p["DEMO-FIB3"]
	cbl, cne, cro := p["DEMO-CBL"], p["DEMO-CNE"], p["DEMO-CRO"]
	ins := func(xs ...any) []muebles.Insumo {
		var out []muebles.Insumo
		for i := 0; i < len(xs); i += 2 {
			out = append(out, muebles.Insumo{ProductoID: p[xs[i].(string)], Cantidad: xs[i+1].(float64)})
		}
		return out
	}

	// 1. Bajo mesada y alacena para Martín: queda como presupuesto para editar.
	bajo := muebles.Mueble{Nombre: "Bajo mesada 2 puertas", AnchoMM: 800, AltoMM: 860, ProfundidadMM: 580, Cantidad: 1,
		Piezas: []muebles.Pieza{
			pz("Lateral", bl, 860, 560, 2, unLargo, cbl, false),
			pz("Piso", bl, 764, 560, 1, unLargo, cbl, false),
			pz("Travesaño", bl, 764, 100, 2, unLargo, cbl, false),
			pz("Estante", bl, 760, 500, 1, unLargo, cbl, false),
			pz("Puerta", bl, 856, 396, 2, todos, cbl, false),
			pz("Fondo", fib, 856, 796, 1, sinCanto, 0, false),
		},
		Insumos: ins("DEMO-BIS35", 4.0, "DEMO-PATA", 4.0, "DEMO-MAN128", 2.0, "DEMO-SOP", 4.0, "DEMO-TOR", 1.0),
	}
	alacena := muebles.Mueble{Nombre: "Alacena 2 puertas", AnchoMM: 800, AltoMM: 600, ProfundidadMM: 320, Cantidad: 1,
		Piezas: []muebles.Pieza{
			pz("Lateral", bl, 600, 300, 2, unLargo, cbl, false),
			pz("Techo y piso", bl, 764, 300, 2, unLargo, cbl, false),
			pz("Estante", bl, 760, 280, 1, unLargo, cbl, false),
			pz("Puerta", bl, 596, 396, 2, todos, cbl, false),
			pz("Fondo", fib, 596, 796, 1, sinCanto, 0, false),
		},
		Insumos: ins("DEMO-BIS35", 4.0, "DEMO-MAN128", 2.0, "DEMO-SOP", 4.0),
	}
	if _, err := d.ventas.CrearPresupuestoMuebles(d.ctx, d.u, ventas.DatosVenta{ClienteID: clientes["Martín Gómez"],
		Notas: "Incluye colocación", FechaEntrega: fecha(25)}, []muebles.Mueble{bajo, alacena}); err != nil {
		return err
	}

	// 2. Placard en roble para Lucía: confirmado con seña, queda el corte pendiente.
	placard := muebles.Mueble{Nombre: "Placard 2 puertas corredizas", AnchoMM: 1600, AltoMM: 2400, ProfundidadMM: 600, Cantidad: 1,
		Piezas: []muebles.Pieza{
			pz("Lateral", ro, 2400, 580, 2, unLargo, cro, false),
			pz("Techo y piso", ro, 580, 1564, 2, unLargo, cro, true),
			pz("Puerta corrediza", ro, 2350, 790, 2, todos, cro, false),
			pz("Estante", bl, 1560, 550, 3, unLargo, cbl, false),
			pz("División", bl, 1800, 550, 1, unLargo, cbl, false),
			pz("Fondo", fib, 2396, 1596, 1, sinCanto, 0, false),
		},
		Insumos: ins("DEMO-RIEL", 1.0, "DEMO-SOP", 12.0, "DEMO-TOR", 1.0),
	}
	p2, err := d.ventas.CrearPresupuestoMuebles(d.ctx, d.u, ventas.DatosVenta{ClienteID: clientes["Lucía Fernández"], FechaEntrega: fecha(20)}, []muebles.Mueble{placard})
	if err != nil {
		return err
	}
	if _, err := d.ventas.Confirmar(d.ctx, d.u, p2.Venta.ID, d.dep1); err != nil {
		return err
	}
	if _, err := d.ventas.RegistrarPago(d.ctx, d.u, p2.Venta.ID, ventas.Pago{Tipo: ventas.PagoSena, Monto: redondearMil(p2.Venta.Total / 2), Medio: "transferencia"}); err != nil {
		return err
	}

	// 3. Biblioteca negra para Ana: confirmada, cortada y en armado.
	biblio := muebles.Mueble{Nombre: "Biblioteca 5 estantes", AnchoMM: 900, AltoMM: 1800, ProfundidadMM: 300, Cantidad: 2,
		Piezas: []muebles.Pieza{
			pz("Lateral", ne, 1800, 300, 2, unLargo, cne, false),
			pz("Techo y piso", ne, 864, 300, 2, unLargo, cne, false),
			pz("Estante", ne, 860, 280, 3, unLargo, cne, false),
		},
		Insumos: ins("DEMO-SOP", 12.0, "DEMO-TOR", 1.0),
	}
	p3, err := d.ventas.CrearPresupuestoMuebles(d.ctx, d.u, ventas.DatosVenta{ClienteID: clientes["Ana Ruiz"], FechaEntrega: fecha(7)}, []muebles.Mueble{biblio})
	if err != nil {
		return err
	}
	if _, err := d.ventas.Confirmar(d.ctx, d.u, p3.Venta.ID, d.dep1); err != nil {
		return err
	}
	if _, err := d.ventas.RegistrarPago(d.ctx, d.u, p3.Venta.ID, ventas.Pago{Tipo: ventas.PagoPago, Monto: p3.Venta.Total, Medio: "efectivo"}); err != nil {
		return err
	}
	cs, err := d.cortes.ListarCortes(d.ctx, d.u, cortes.CortePendiente)
	if err != nil {
		return err
	}
	for _, c := range cs {
		if c.PedidoID != nil && *c.PedidoID == p3.Venta.ID {
			if _, err := d.cortes.ConfirmarCorte(d.ctx, d.u, c.ID); err != nil {
				return err
			}
		}
	}
	for _, e := range []ventas.Estado{ventas.EnCorte, ventas.EnArmado} {
		if _, err := d.ventas.CambiarEstado(d.ctx, d.u, p3.Venta.ID, e); err != nil {
			return err
		}
	}

	// 4. Un corte suelto desde el optimizador, pendiente, con veta vertical.
	_, err = d.cortes.CrearCorte(d.ctx, d.u, cortes.CorteNuevo{
		Nombre: "Frentes de cajonera (taller)", PlacaID: ro, DepositoID: d.dep1, Kerf: 3,
		Piezas: []cortes.Pieza{
			{ID: "1", Etiqueta: "Frente cajón", Largo: 596, Ancho: 180, Cantidad: 4, RespetaVeta: true, Cantos: todos},
			{ID: "2", Etiqueta: "Tapa", Largo: 1200, Ancho: 450, Cantidad: 1, RespetaVeta: true, Cantos: unLargo},
			{ID: "3", Etiqueta: "Lateral", Largo: 720, Ancho: 450, Cantidad: 2, RespetaVeta: true, VetaVertical: true, Cantos: unLargo},
		},
	})
	return err
}

func redondearMil(x float64) float64 { return float64(int64(x/1000)) * 1000 }
