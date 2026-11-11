package ventas

import (
	"errors"
	"testing"
	"time"

	"github.com/FedeCF03/nandubay/backend/internal/modelos/stock"
)

func f(v float64) *float64 { return &v }

var productos = map[int64]stock.Producto{
	1: {ID: 1, Tipo: stock.TipoPlaca, Nombre: "Melamina blanca 18 mm", PrecioVenta: 50000, Costo: 35000},
	2: {ID: 2, Tipo: stock.TipoCanto, Nombre: "Canto blanco 22 mm", PrecioVenta: 300, Costo: 150},
	3: {ID: 3, Tipo: stock.TipoHerraje, Nombre: "Bisagra cazoleta", PrecioVenta: 1200, Costo: 600},
}

func venta(t *testing.T, estado Estado, nuevas ...LineaNueva) Pedido {
	t.Helper()
	ls, err := ArmarLineas(nuevas, productos)
	if err != nil {
		t.Fatal(err)
	}
	for i := range ls {
		ls[i].ID = int64(i + 1)
	}
	gonnet := Lugar{ID: 10, Nombre: "Gonnet"}
	laPlata := Lugar{ID: 20, Nombre: "La Plata"}
	dep := int64(200)
	return Pedido{ID: 1, Numero: 7, Tipo: Mostrador, Estado: estado, Sucursal: gonnet, DepositoID: &dep, Retira: &laPlata, Lineas: ls}
}

func TestArmarLineas(t *testing.T) {
	ls, err := ArmarLineas([]LineaNueva{
		{Tipo: LineaProducto, ProductoID: 1, Cantidad: 2},
		{Tipo: LineaProducto, ProductoID: 2, Cantidad: 12.5, PrecioUnitario: f(280)},
		{Tipo: LineaMueble, Descripcion: "Bajo mesada 1,20 m", AnchoMM: 1200, AltoMM: 850, ProfundidadMM: 600, Cantidad: 1, PrecioUnitario: f(450000)},
	}, productos)
	if err != nil {
		t.Fatal(err)
	}
	if ls[0].PrecioUnitario != 50000 || ls[0].CostoUnitario != 35000 || ls[0].Descripcion != "Melamina blanca 18 mm" {
		t.Errorf("no tomó los datos del producto: %+v", ls[0])
	}
	if ls[1].PrecioUnitario != 280 || ls[1].Unidad != "metro" {
		t.Errorf("no respetó el precio cargado: %+v", ls[1])
	}
	p := Pedido{Lineas: ls}
	if got := p.Total(); got != 100000+3500+450000 {
		t.Errorf("total = %v", got)
	}

	malos := []LineaNueva{
		{Tipo: LineaProducto, ProductoID: 99, Cantidad: 1},
		{Tipo: LineaProducto, ProductoID: 1, Cantidad: 0},
		{Tipo: LineaProducto, ProductoID: 1, Cantidad: 1.5},      // placas enteras
		{Tipo: LineaMueble, Cantidad: 1, PrecioUnitario: f(10)},  // sin descripción
		{Tipo: LineaMueble, Descripcion: "Alacena", Cantidad: 1}, // sin precio
		{Tipo: LineaProducto, ProductoID: 1, Cantidad: 1, PrecioUnitario: f(-1)},
		{Tipo: "otra", Cantidad: 1},
	}
	for _, m := range malos {
		var ev ErrValidacion
		if _, err := ArmarLineas([]LineaNueva{m}, productos); !errors.As(err, &ev) {
			t.Errorf("%+v: esperaba error de validación, salió %v", m, err)
		}
	}
	if _, err := ArmarLineas(nil, productos); !errors.Is(err, ErrSinLineas) {
		t.Errorf("sin líneas: %v", err)
	}
}

var hoy = time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)

func TestConfirmarMostradorNoDescuentaStock(t *testing.T) {
	p := venta(t, Presupuesto,
		LineaNueva{Tipo: LineaProducto, ProductoID: 1, Cantidad: 2},
		LineaNueva{Tipo: LineaProducto, ProductoID: 1, Cantidad: 1},
		LineaNueva{Tipo: LineaProducto, ProductoID: 3, Cantidad: 4},
	)
	c, err := p.Confirmar(300, nil, hoy)
	if err != nil {
		t.Fatal(err)
	}
	if *c.Estado != Confirmado || *c.DepositoID != 300 {
		t.Errorf("cambio = %+v", c)
	}
	if len(c.Operaciones) != 0 {
		t.Errorf("en el mostrador el stock se descuenta al retirar, no al confirmar: %+v", c.Operaciones)
	}
	// Tiene que haber disponible lo que se vende, sumado por producto.
	if len(c.Disponible) != 2 || c.Disponible[0] != (Consumo{1, 3}) || c.Disponible[1] != (Consumo{3, 4}) {
		t.Errorf("disponible = %+v", c.Disponible)
	}
	if c.ReservarHasta != nil {
		t.Error("sin pagos no se reserva")
	}
	p.Estado = Confirmado
	if _, err := p.Confirmar(300, nil, hoy); !errors.Is(err, ErrEstado) {
		t.Errorf("confirmar dos veces: %v", err)
	}
}

func TestReservaAlPagar(t *testing.T) {
	// Pagado antes de confirmar: se reserva al confirmar.
	p := venta(t, Presupuesto, LineaNueva{Tipo: LineaProducto, ProductoID: 1, Cantidad: 2})
	p.Pagos = []Pago{{Tipo: PagoSena, Monto: 1000}}
	c, err := p.Confirmar(200, nil, hoy)
	if err != nil || c.ReservarHasta == nil || !c.ReservarHasta.Equal(hoy.AddDate(0, 2, 0)) {
		t.Fatalf("confirmar con seña reserva 2 meses: %v %v", c.ReservarHasta, err)
	}

	// Confirmada sin pagar: la seña reserva lo que falta retirar.
	p = venta(t, Confirmado, LineaNueva{Tipo: LineaProducto, ProductoID: 1, Cantidad: 2})
	if p.Reservada(hoy) {
		t.Fatal("sin pagos no está reservada")
	}
	c, err = p.RegistrarPago(Pago{Tipo: PagoSena, Monto: 30000, Medio: "efectivo"}, hoy)
	if err != nil || c.ReservarHasta == nil || len(c.Disponible) != 1 || c.Disponible[0] != (Consumo{1, 2}) {
		t.Fatalf("la seña reserva: %+v %v", c, err)
	}
	p.ReservadoHasta = c.ReservarHasta
	if !p.Reservada(hoy.AddDate(0, 1, 0)) || p.Reservada(hoy.AddDate(0, 2, 1)) {
		t.Error("la reserva dura 2 meses")
	}
	// Otro pago con la reserva vigente no la estira.
	c, _ = p.RegistrarPago(Pago{Tipo: PagoPago, Monto: 1000, Medio: "efectivo"}, hoy.AddDate(0, 1, 0))
	if c.ReservarHasta != nil || c.Disponible != nil {
		t.Errorf("con reserva vigente no se vuelve a reservar: %+v", c)
	}
	// Vencida, un pago nuevo reserva otra vez.
	c, _ = p.RegistrarPago(Pago{Tipo: PagoPago, Monto: 1000, Medio: "efectivo"}, hoy.AddDate(0, 3, 0))
	if c.ReservarHasta == nil {
		t.Error("con la reserva vencida un pago reserva de nuevo")
	}
	// Una devolución no reserva.
	p.ReservadoHasta = nil
	p.Pagos = []Pago{{Tipo: PagoSena, Monto: 5000}}
	if c, _ := p.RegistrarPago(Pago{Tipo: PagoDevolucion, Monto: 100, Medio: "efectivo"}, hoy); c.ReservarHasta != nil {
		t.Error("una devolución no reserva")
	}
	// Un presupuesto de muebles no reserva: sus materiales salen al confirmar.
	p.Tipo = Muebles
	if c, _ := p.RegistrarPago(Pago{Tipo: PagoSena, Monto: 100, Medio: "efectivo"}, hoy); c.ReservarHasta != nil {
		t.Error("muebles no reserva")
	}
}

func TestPagos(t *testing.T) {
	p := venta(t, Confirmado, LineaNueva{Tipo: LineaProducto, ProductoID: 1, Cantidad: 2}) // $100.000
	if p.EstaPagada() {
		t.Fatal("sin pagos no está pagada")
	}
	c, err := p.RegistrarPago(Pago{Tipo: PagoSena, Monto: 30000, Medio: "efectivo"}, hoy)
	if err != nil {
		t.Fatal(err)
	}
	p.Pagos = append(p.Pagos, *c.Pago)
	if p.Saldo() != 70000 {
		t.Errorf("saldo = %v", p.Saldo())
	}
	if _, err := p.RegistrarPago(Pago{Tipo: PagoPago, Monto: 70000.01, Medio: "efectivo"}, hoy); !errors.Is(err, ErrPagoExcedido) {
		t.Errorf("pago de más: %v", err)
	}
	if _, err := p.RegistrarPago(Pago{Tipo: PagoPago, Monto: 10, Medio: " "}, hoy); err == nil {
		t.Error("sin medio de pago")
	}
	if _, err := p.RegistrarPago(Pago{Tipo: PagoDevolucion, Monto: 40000, Medio: "efectivo"}, hoy); err == nil {
		t.Error("devolver más de lo pagado")
	}
	c, _ = p.RegistrarPago(Pago{Tipo: PagoPago, Monto: 70000, Medio: "transferencia"}, hoy)
	p.Pagos = append(p.Pagos, *c.Pago)
	if !p.EstaPagada() {
		t.Error("tendría que estar pagada")
	}
	p.Estado = Presupuesto
	if p.EstaPagada() {
		t.Error("un presupuesto nunca dice pagada")
	}
}

func TestRetiroPorPartesEnLaSucursalDelDeposito(t *testing.T) {
	p := venta(t, Confirmado,
		LineaNueva{Tipo: LineaProducto, ProductoID: 1, Cantidad: 3},
		LineaNueva{Tipo: LineaProducto, ProductoID: 2, Cantidad: 10},
	)
	items := []RetiroItem{{LineaID: 1, Tipo: LineaProducto, Cantidad: 2}}

	if _, err := p.Retirar(20, items, ""); !errors.Is(err, ErrSaldoPendiente) {
		t.Errorf("sin pagar no retira: %v", err)
	}
	p.Pagos = []Pago{{Tipo: PagoPago, Monto: p.Total()}}

	if _, err := p.Retirar(10, items, ""); !errors.Is(err, ErrOtraSucursal) {
		t.Errorf("se vendió en Gonnet pero se retira en La Plata: %v", err)
	}

	c, err := p.Retirar(20, items, "se lleva 2 ahora")
	if err != nil {
		t.Fatal(err)
	}
	if c.Estado != nil {
		t.Error("falta retirar, no está entregada")
	}
	if len(c.Retiro.Items) != 1 || c.Retiro.Items[0].Cantidad != 2 || c.Retiro.SucursalID != 20 {
		t.Errorf("retiro = %+v", c.Retiro)
	}
	// El stock sale del depósito de la venta al retirar.
	if len(c.Operaciones) != 1 || c.Operaciones[0].ProductoID != 1 || c.Operaciones[0].Cantidad != -2 ||
		c.Operaciones[0].DepositoID != 200 || c.Operaciones[0].Tipo != stock.MovVenta {
		t.Errorf("operaciones del retiro = %+v", c.Operaciones)
	}
	p.Lineas[0].Retirado = 2
	p.Retiros = []Retiro{{ID: 1}}

	if _, err := p.Retirar(20, []RetiroItem{{LineaID: 1, Tipo: LineaProducto, Cantidad: 2}}, ""); err == nil {
		t.Error("no puede retirar más de lo que queda")
	}
	if _, err := p.Retirar(20, []RetiroItem{{LineaID: 1, Tipo: LineaProducto, Cantidad: 0}}, ""); !errors.Is(err, ErrNadaQueRetirar) {
		t.Errorf("retiro vacío: %v", err)
	}
	if _, err := p.Retirar(20, []RetiroItem{{LineaID: 9, Tipo: LineaProducto, Cantidad: 1}}, ""); err == nil {
		t.Error("renglón de otra venta")
	}
	if _, err := p.Retirar(20, []RetiroItem{{LineaID: 1, Tipo: LineaMueble, Cantidad: 1}}, ""); err == nil {
		t.Error("el id 1 es de un producto, no de un mueble")
	}

	c, err = p.Retirar(20, []RetiroItem{{LineaID: 1, Tipo: LineaProducto, Cantidad: 1}, {LineaID: 2, Tipo: LineaProducto, Cantidad: 10}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if c.Estado == nil || *c.Estado != Entregado {
		t.Error("con todo retirado queda entregada")
	}
	if len(c.Operaciones) != 2 || c.Operaciones[1].ProductoID != 2 || c.Operaciones[1].Cantidad != -10 {
		t.Errorf("operaciones del segundo retiro = %+v", c.Operaciones)
	}

	p.Estado = Entregado
	if _, err := p.Retirar(20, items, ""); !errors.Is(err, ErrEstado) {
		t.Errorf("una venta entregada no se vuelve a retirar: %v", err)
	}
}

func TestMueblesSeRetiranCuandoEstanListos(t *testing.T) {
	p := venta(t, EnArmado, LineaNueva{Tipo: LineaMueble, Descripcion: "Alacena", Cantidad: 2, PrecioUnitario: f(1000)})
	p.Pagos = []Pago{{Tipo: PagoPago, Monto: 2000}}
	if _, err := p.Retirar(20, []RetiroItem{{LineaID: 1, Tipo: LineaMueble, Cantidad: 1}}, ""); !errors.Is(err, ErrMuebleNoListo) {
		t.Errorf("en armado: %v", err)
	}
	p.Estado = Listo
	if _, err := p.Retirar(20, []RetiroItem{{LineaID: 1, Tipo: LineaMueble, Cantidad: 1}}, ""); err != nil {
		t.Error(err)
	}
}

func TestEstados(t *testing.T) {
	p := venta(t, Presupuesto, LineaNueva{Tipo: LineaProducto, ProductoID: 3, Cantidad: 1})
	if _, err := p.CambiarEstado(EnCorte); !errors.Is(err, ErrEstado) {
		t.Error("un presupuesto no pasa a producción sin confirmar")
	}
	p.Estado = Confirmado
	if c, err := p.CambiarEstado(Listo); err != nil || *c.Estado != Listo {
		t.Errorf("confirmado → listo: %v", err)
	}
	if _, err := p.CambiarEstado(Entregado); !errors.Is(err, ErrEstado) {
		t.Error("entregada no se pone a mano")
	}
	if _, err := p.Editar(DatosVenta{}, p.Lineas); !errors.Is(err, ErrEstado) {
		t.Error("una venta confirmada no se edita")
	}
}

func TestCancelar(t *testing.T) {
	p := venta(t, Presupuesto, LineaNueva{Tipo: LineaProducto, ProductoID: 1, Cantidad: 2})
	c, err := p.Cancelar()
	if err != nil || len(c.Operaciones) != 0 {
		t.Errorf("un presupuesto se cancela sin tocar stock: %+v %v", c, err)
	}

	p.Estado = Confirmado
	c, err = p.Cancelar()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Operaciones) != 0 {
		t.Errorf("en el mostrador no salió nada del stock, no hay nada que devolver: %+v", c.Operaciones)
	}

	p.Pagos = []Pago{{Tipo: PagoSena, Monto: 100}}
	if _, err := p.Cancelar(); !errors.Is(err, ErrConPagos) {
		t.Errorf("con pagos: %v", err)
	}
	p.Pagos = append(p.Pagos, Pago{Tipo: PagoDevolucion, Monto: 100})
	p.Retiros = []Retiro{{ID: 1}}
	if _, err := p.Cancelar(); !errors.Is(err, ErrConRetiros) {
		t.Errorf("con retiros: %v", err)
	}
}

func TestCliente(t *testing.T) {
	c := Cliente{Nombre: "  María Gómez ", Email: " MARIA@Ejemplo.com "}
	if err := c.Normalizar(); err != nil || c.Nombre != "María Gómez" || c.Email != "maria@ejemplo.com" {
		t.Errorf("%+v %v", c, err)
	}
	if err := (&Cliente{Nombre: " "}).Normalizar(); err == nil {
		t.Error("sin nombre")
	}
	if err := (&Cliente{Nombre: "x", Email: "nada"}).Normalizar(); err == nil {
		t.Error("email inválido")
	}
}

func TestMostradorSoloProductos(t *testing.T) {
	ls, _ := ArmarLineas([]LineaNueva{
		{Tipo: LineaProducto, ProductoID: 1, Cantidad: 1},
		{Tipo: LineaMueble, Descripcion: "Alacena", Cantidad: 1, PrecioUnitario: f(10)},
	}, productos)
	if err := ControlarLineas(Mostrador, ls[:1]); err != nil {
		t.Error(err)
	}
	if err := ControlarLineas(Mostrador, ls); err == nil {
		t.Error("un mueble no se vende en el mostrador")
	}
	if err := ControlarLineas(Muebles, ls[1:]); err != nil {
		t.Error(err)
	}
	if err := ControlarLineas(Muebles, ls); err == nil {
		t.Error("un producto suelto no va en un presupuesto de muebles")
	}
	p := Pedido{Tipo: Mostrador, Estado: Presupuesto}
	if _, err := p.Editar(DatosVenta{}, ls); err == nil {
		t.Error("editar un presupuesto de mostrador con un mueble")
	}
}

func TestConfirmarMueblesDescuentaMateriales(t *testing.T) {
	p := venta(t, Presupuesto,
		LineaNueva{Tipo: LineaMueble, Descripcion: "Alacena", Cantidad: 2, PrecioUnitario: f(1000)},
		LineaNueva{Tipo: LineaProducto, ProductoID: 3, Cantidad: 4},
	)
	p.Tipo = Muebles
	c, err := p.Confirmar(300, []Consumo{{1, 2}, {3, 6}, {4, 12.5}}, hoy)
	if err != nil {
		t.Fatal(err)
	}
	got := map[int64]float64{}
	for _, op := range c.Operaciones {
		got[op.ProductoID] = -op.Cantidad
	}
	if len(c.Operaciones) != 3 || got[1] != 2 || got[3] != 10 || got[4] != 12.5 {
		t.Errorf("operaciones = %v", got)
	}
	// Al cancelar vuelve lo que se descontó.
	p.Estado, p.DepositoID = Confirmado, ptr(int64(300))
	p.Consumido = []Consumo{{1, 2}, {3, 10}, {4, 12.5}}
	c, err = p.Cancelar()
	if err != nil || len(c.Operaciones) != 3 || c.Operaciones[2].Cantidad != 12.5 || c.Operaciones[2].Tipo != stock.MovDevolucion {
		t.Errorf("cancelar: %v %+v", err, c.Operaciones)
	}
}
