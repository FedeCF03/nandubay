package aplicacion

import (
	"bytes"
	"context"
	"errors"
	"net/http"

	"github.com/FedeCF03/nandubay/backend/internal/dominio/acceso"
	"github.com/FedeCF03/nandubay/backend/internal/dominio/muebles"
	"github.com/FedeCF03/nandubay/backend/internal/dominio/ventas"
)

// KerfMM es el espesor de sierra con que se calculan los presupuestos.
const KerfMM = 3

// PresupuestoMuebles es un presupuesto de muebles con su despiece y los
// materiales calculados.
type PresupuestoMuebles struct {
	Venta      VentaVista          `json:"venta"`
	Muebles    []muebles.Mueble    `json:"muebles"`
	Materiales *muebles.Materiales `json:"materiales"`
	Fotos      []FotoMueble        `json:"fotos"`
	// ErrorMateriales dice por qué no se pudieron calcular (por ejemplo, si
	// se dio de baja un producto).
	ErrorMateriales string `json:"error_materiales,omitempty"`
}

// calcular valida los muebles contra el stock y calcula los materiales.
func (s *ServicioVentas) calcular(ctx context.Context, u *acceso.Usuario, ms []muebles.Mueble) (muebles.Materiales, error) {
	productos, err := s.Repo.BuscarProductos(ctx, u.CarpinteriaID, muebles.ProductosUsados(ms))
	if err != nil {
		return muebles.Materiales{}, err
	}
	if err := muebles.Validar(ms, productos); err != nil {
		return muebles.Materiales{}, err
	}
	return muebles.Calcular(ms, productos, KerfMM)
}

// CalcularMateriales calcula placas, canto, insumos y costo sin guardar nada,
// para ir viéndolo mientras se arma el presupuesto.
func (s *ServicioVentas) CalcularMateriales(ctx context.Context, u *acceso.Usuario, ms []muebles.Mueble) (muebles.Materiales, error) {
	return s.calcular(ctx, u, ms)
}

func (s *ServicioVentas) CrearPresupuestoMuebles(ctx context.Context, u *acceso.Usuario, d ventas.DatosVenta, ms []muebles.Mueble) (PresupuestoMuebles, error) {
	mat, err := s.calcular(ctx, u, ms)
	if err != nil {
		return PresupuestoMuebles{}, err
	}
	id, err := s.Repo.CrearVenta(ctx, u.CarpinteriaID, VentaNueva{
		Tipo: ventas.Muebles, SucursalID: u.SucursalID, VendedorID: u.ID, Datos: limpiarDatos(d),
		Lineas: ventas.LineasDeMuebles(ms, mat.CostoPorMueble), Despiece: ms,
	})
	if err != nil {
		return PresupuestoMuebles{}, err
	}
	return s.VerPresupuestoMuebles(ctx, u, id)
}

func (s *ServicioVentas) EditarPresupuestoMuebles(ctx context.Context, u *acceso.Usuario, id int64, d ventas.DatosVenta, ms []muebles.Mueble) (PresupuestoMuebles, error) {
	mat, err := s.calcular(ctx, u, ms)
	if err != nil {
		return PresupuestoMuebles{}, err
	}
	d = limpiarDatos(d)
	lineas := ventas.LineasDeMuebles(ms, mat.CostoPorMueble)
	err = s.Repo.Modificar(ctx, u.CarpinteriaID, u.ID, id, func(p ventas.Pedido) (ventas.Cambio, error) {
		if p.Tipo != ventas.Muebles {
			return ventas.Cambio{}, ventas.ErrNoEncontrado
		}
		c, err := p.Editar(d, lineas)
		c.Despiece = ms
		return c, err
	})
	if err != nil {
		return PresupuestoMuebles{}, err
	}
	return s.VerPresupuestoMuebles(ctx, u, id)
}

func (s *ServicioVentas) VerPresupuestoMuebles(ctx context.Context, u *acceso.Usuario, id int64) (PresupuestoMuebles, error) {
	v, err := s.VerVenta(ctx, u, id)
	if err != nil {
		return PresupuestoMuebles{}, err
	}
	if v.Tipo != ventas.Muebles {
		return PresupuestoMuebles{}, ventas.ErrNoEncontrado
	}
	ms, err := s.Repo.BuscarDespiece(ctx, u.CarpinteriaID, id)
	if err != nil {
		return PresupuestoMuebles{}, err
	}
	fotos, err := s.Repo.ListarFotos(ctx, u.CarpinteriaID, id)
	if err != nil {
		return PresupuestoMuebles{}, err
	}
	out := PresupuestoMuebles{Venta: v, Muebles: ms, Fotos: fotos}
	if len(ms) > 0 {
		if mat, err := s.calcular(ctx, u, ms); err != nil {
			out.ErrorMateriales = err.Error()
		} else {
			out.Materiales = &mat
		}
	}
	return out, nil
}

// Límites de las fotos de los muebles. El navegador las achica antes de
// mandarlas, así que una foto normal pesa unos cientos de KB.
const (
	MaxFotosPorMueble = 12
	MaxBytesFoto      = 4 << 20
)

var (
	ErrDemasiadasFotos = errors.New("el mueble ya tiene el máximo de fotos")
	ErrFotoInvalida    = errors.New("la foto tiene que ser JPG, PNG o WebP de hasta 4 MB")
)

// AgregarFoto guarda una foto de un mueble del presupuesto. El tipo se
// detecta de los bytes, no se le cree al navegador.
func (s *ServicioVentas) AgregarFoto(ctx context.Context, u *acceso.Usuario, pedidoID int64, clave string, datos []byte) (FotoMueble, error) {
	if len(datos) == 0 || len(datos) > MaxBytesFoto {
		return FotoMueble{}, ErrFotoInvalida
	}
	tipo := http.DetectContentType(datos)
	// DetectContentType no reconoce WebP en todas las versiones: se mira la cabecera.
	if len(datos) > 12 && bytes.Equal(datos[:4], []byte("RIFF")) && bytes.Equal(datos[8:12], []byte("WEBP")) {
		tipo = "image/webp"
	}
	switch tipo {
	case "image/jpeg", "image/png", "image/webp":
	default:
		return FotoMueble{}, ErrFotoInvalida
	}
	if _, err := s.verPresupuesto(ctx, u, pedidoID); err != nil {
		return FotoMueble{}, err
	}
	return s.Repo.AgregarFoto(ctx, u.CarpinteriaID, u.ID, pedidoID, FotoNueva{MuebleClave: clave, Tipo: tipo, Datos: datos})
}

func (s *ServicioVentas) VerFoto(ctx context.Context, u *acceso.Usuario, pedidoID, fotoID int64) (string, []byte, error) {
	return s.Repo.BuscarFoto(ctx, u.CarpinteriaID, pedidoID, fotoID)
}

func (s *ServicioVentas) BorrarFoto(ctx context.Context, u *acceso.Usuario, pedidoID, fotoID int64) error {
	return s.Repo.BorrarFoto(ctx, u.CarpinteriaID, pedidoID, fotoID)
}

// verPresupuesto controla que la venta exista y sea un presupuesto de muebles.
func (s *ServicioVentas) verPresupuesto(ctx context.Context, u *acceso.Usuario, id int64) (VentaVista, error) {
	v, err := s.VerVenta(ctx, u, id)
	if err == nil && v.Tipo != ventas.Muebles {
		err = ventas.ErrNoEncontrado
	}
	return v, err
}

// materialesAConsumir calcula, para un presupuesto de muebles, las placas, el
// canto y los insumos que salen del stock al confirmarlo. En una venta de
// mostrador no hay nada extra: alcanza con los renglones.
func (s *ServicioVentas) materialesAConsumir(ctx context.Context, u *acceso.Usuario, id int64) ([]ventas.Consumo, error) {
	v, err := s.VerVenta(ctx, u, id)
	if err != nil || v.Tipo != ventas.Muebles {
		return nil, err
	}
	ms, err := s.Repo.BuscarDespiece(ctx, u.CarpinteriaID, id)
	if err != nil {
		return nil, err
	}
	conDespiece := false
	for _, m := range ms {
		if len(m.Piezas) > 0 || len(m.Insumos) > 0 {
			conDespiece = true
		}
	}
	if !conDespiece {
		return nil, nil
	}
	mat, err := s.calcular(ctx, u, ms)
	if err != nil {
		return nil, err
	}
	var out []ventas.Consumo
	for _, p := range mat.Placas {
		out = append(out, ventas.Consumo{ProductoID: p.ProductoID, Cantidad: float64(p.Cantidad)})
	}
	for _, c := range append(mat.Cantos, mat.Insumos...) {
		out = append(out, ventas.Consumo{ProductoID: c.ProductoID, Cantidad: c.Cantidad})
	}
	return out, nil
}
