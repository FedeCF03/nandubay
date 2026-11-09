package acceso

// RolBase es un rol que se crea con cada carpintería nueva.
type RolBase struct {
	Nombre   string
	Todos    bool     // true = todos los permisos del sistema
	Permisos []string // si Todos es false
}

// RolesBase son los roles que trae cada carpintería al darse de alta.
// Después cada una los puede cambiar o armar roles nuevos.
var RolesBase = []RolBase{
	{Nombre: "Admin", Todos: true},
	{Nombre: "Vendedor", Permisos: []string{
		"productos.ver", "stock.ver", "clientes.ver", "clientes.editar",
		"pedidos.ver", "pedidos.editar", "pedidos.confirmar", "pagos.registrar",
		"cortes.ver", "cortes.generar",
	}},
	{Nombre: "Operario", Permisos: []string{
		"pedidos.ver", "pedidos.estado", "cortes.ver", "cortes.generar", "stock.ver",
	}},
	{Nombre: "Depósito", Permisos: []string{
		"productos.ver", "productos.editar", "stock.ver", "stock.editar",
		"compras.ver", "compras.editar", "cortes.ver",
	}},
}

// NombreRolAdmin es el rol que recibe el primer usuario de una carpintería.
const NombreRolAdmin = "Admin"
