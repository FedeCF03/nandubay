package cortes

// MetrosCanto calcula los metros de tapacanto necesarios para las piezas.
// mermaPorTramo se suma a cada lado con canto (lo que sobra en las puntas al
// pegarlo), en milímetros.
func MetrosCanto(piezas []Pieza, mermaPorTramo int) float64 {
	var mm int
	for _, p := range piezas {
		cant := p.Cantidad
		if cant <= 0 {
			cant = 1
		}
		for _, tiene := range []struct {
			ok  bool
			len int
		}{{p.Cantos.L1, p.Largo}, {p.Cantos.L2, p.Largo}, {p.Cantos.A1, p.Ancho}, {p.Cantos.A2, p.Ancho}} {
			if tiene.ok {
				mm += (tiene.len + mermaPorTramo) * cant
			}
		}
	}
	return redondear(float64(mm) / 1000)
}
