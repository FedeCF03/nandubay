package api

import (
	"net/http"

	"github.com/FedeCF03/nandubay/backend/internal/dominio/cortes"
)

type pedidoOptimizar struct {
	Parametros   cortes.Parametros `json:"parametros"`
	Piezas       []cortes.Pieza    `json:"piezas"`
	MermaCantoMM int               `json:"merma_canto_mm"`
}

func (s *Servidor) optimizarCortes(w http.ResponseWriter, r *http.Request) {
	var p pedidoOptimizar
	if !leerJSON(w, r, &p) {
		return
	}
	if len(p.Piezas) == 0 {
		responderError(w, http.StatusBadRequest, "no hay piezas para cortar")
		return
	}
	plan, err := s.Cortes.Optimizar(p.Parametros, p.Piezas, p.MermaCantoMM)
	if err != nil {
		responderError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	responderJSON(w, http.StatusOK, plan)
}
