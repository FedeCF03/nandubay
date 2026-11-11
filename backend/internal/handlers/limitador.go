package handlers

import (
	"sync"
	"time"
)

// Limitador frena los intentos de login repetidos desde una misma IP.
type Limitador struct {
	mu       sync.Mutex
	max      int
	ventana  time.Duration
	intentos map[string][]time.Time
}

func NuevoLimitador(max int, ventana time.Duration) *Limitador {
	return &Limitador{max: max, ventana: ventana, intentos: map[string][]time.Time{}}
}

// Permitir registra un intento y dice si todavía está dentro del límite.
func (l *Limitador) Permitir(clave string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	ahora := time.Now()
	desde := ahora.Add(-l.ventana)
	vigentes := l.intentos[clave][:0]
	for _, t := range l.intentos[clave] {
		if t.After(desde) {
			vigentes = append(vigentes, t)
		}
	}
	if len(vigentes) >= l.max {
		l.intentos[clave] = vigentes
		return false
	}
	l.intentos[clave] = append(vigentes, ahora)
	if len(l.intentos) > 10000 { // no dejar crecer el mapa sin límite
		for k, v := range l.intentos {
			if len(v) == 0 || v[len(v)-1].Before(desde) {
				delete(l.intentos, k)
			}
		}
	}
	return true
}

// Limpiar borra los intentos de una clave (por ejemplo, tras un login correcto).
func (l *Limitador) Limpiar(clave string) {
	l.mu.Lock()
	delete(l.intentos, clave)
	l.mu.Unlock()
}
