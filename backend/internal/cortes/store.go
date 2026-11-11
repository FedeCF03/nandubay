package cortes

import (
	"context"

	"github.com/FedeCF03/nandubay/backend/internal/stock"
)

// Store guarda los cortes optimizados.
type Store interface {
	BuscarProductos(ctx context.Context, carpinteriaID int64, ids []int64) (map[int64]stock.Producto, error)
	RetazosDisponibles(ctx context.Context, carpinteriaID int64, placaIDs []int64, depositoID int64) (map[int64][]Retazo, error)
	ListarCortes(ctx context.Context, carpinteriaID int64, estado string) ([]CorteOptimizado, error)
	BuscarCorte(ctx context.Context, carpinteriaID, id int64) (CorteOptimizado, error)
	CrearCorte(ctx context.Context, carpinteriaID, usuarioID int64, c CorteOptimizado) (int64, error)
	ActualizarResultado(ctx context.Context, carpinteriaID, id int64, res Resultado) error
	Anular(ctx context.Context, carpinteriaID, id int64) error
	// ConfirmarCorte registra que el corte se hizo: descuenta las placas enteras
	// del depósito, marca los retazos usados y guarda los sobrantes como retazos.
	ConfirmarCorte(ctx context.Context, carpinteriaID, usuarioID, id int64) error
}
