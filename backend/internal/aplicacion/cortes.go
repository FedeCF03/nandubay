package aplicacion

import "github.com/FedeCF03/nandubay/backend/internal/dominio/cortes"

// PlanCortes es el resultado de optimizar, con los metros de canto.
type PlanCortes struct {
	cortes.Resultado
	MetrosCanto float64 `json:"metros_canto"`
}

// ServicioCortes arma planes de corte. Más adelante guarda los planes de un
// pedido y genera los retazos, para lo que va a usar un repositorio.
type ServicioCortes struct{}

func (ServicioCortes) Optimizar(p cortes.Parametros, piezas []cortes.Pieza, mermaCantoMM int) (PlanCortes, error) {
	res, err := cortes.Optimizar(p, piezas)
	if err != nil {
		return PlanCortes{}, err
	}
	return PlanCortes{res, cortes.MetrosCanto(piezas, mermaCantoMM)}, nil
}
