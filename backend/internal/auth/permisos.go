package auth

// Permisos de los roles que se crean con cada carpintería nueva.
// Después cada carpintería los puede cambiar o armar roles nuevos.
var rolesBase = []struct {
	Nombre   string
	Permisos []string
}{
	{"Admin", nil}, // nil = todos los permisos
	{"Vendedor", []string{
		"productos.ver", "stock.ver", "clientes.ver", "clientes.editar",
		"pedidos.ver", "pedidos.editar", "pedidos.confirmar", "pagos.registrar",
		"cortes.ver", "cortes.generar",
	}},
	{"Operario", []string{
		"pedidos.ver", "pedidos.estado", "cortes.ver", "cortes.generar", "stock.ver",
	}},
	{"Depósito", []string{
		"productos.ver", "productos.editar", "stock.ver", "stock.editar",
		"compras.ver", "compras.editar", "cortes.ver",
	}},
}

// Permisos es el conjunto de permisos de un usuario en la sucursal actual.
type Permisos map[string]bool

func (p Permisos) Tiene(codigo string) bool { return p[codigo] }

func (p Permisos) Lista() []string {
	out := make([]string, 0, len(p))
	for k := range p {
		out = append(out, k)
	}
	return out
}
