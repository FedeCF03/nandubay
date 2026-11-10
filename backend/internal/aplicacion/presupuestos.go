package aplicacion

import (
	"context"

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
	out := PresupuestoMuebles{Venta: v, Muebles: ms}
	if len(ms) > 0 {
		if mat, err := s.calcular(ctx, u, ms); err != nil {
			out.ErrorMateriales = err.Error()
		} else {
			out.Materiales = &mat
		}
	}
	return out, nil
}
