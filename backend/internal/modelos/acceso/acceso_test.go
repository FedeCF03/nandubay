package acceso

import (
	"reflect"
	"testing"
)

func ptr(i int64) *int64 { return &i }

var activas = []Sucursal{{ID: 1, Nombre: "Centro"}, {ID: 2, Nombre: "Norte"}, {ID: 3, Nombre: "Sur"}}

func TestRolGlobalVeTodasLasSucursales(t *testing.T) {
	ss, elegida, p, err := ResolverAcceso(activas, []Asignacion{{SucursalID: nil, Permiso: "stock.ver"}}, nil)
	if err != nil || len(ss) != 3 || elegida != 1 || !p.Tiene("stock.ver") {
		t.Fatalf("inesperado: %v %d %v %v", ss, elegida, p, err)
	}
}

func TestPermisosSoloEnSuSucursal(t *testing.T) {
	asig := []Asignacion{{SucursalID: nil, Permiso: "pedidos.ver"}, {SucursalID: ptr(2), Permiso: "pedidos.editar"}}
	_, _, p, _ := ResolverAcceso(activas, asig, ptr(1))
	if p.Tiene("pedidos.editar") || !p.Tiene("pedidos.ver") {
		t.Fatalf("en Centro no debería poder editar: %v", p)
	}
	_, elegida, p, _ := ResolverAcceso(activas, asig, ptr(2))
	if elegida != 2 || !reflect.DeepEqual(p.Lista(), []string{"pedidos.editar", "pedidos.ver"}) {
		t.Fatalf("en Norte debería editar: %d %v", elegida, p)
	}
}

func TestPreferidaAjenaSeIgnora(t *testing.T) {
	ss, elegida, _, _ := ResolverAcceso(activas, []Asignacion{{SucursalID: ptr(3), Permiso: "cortes.ver"}}, ptr(1))
	if len(ss) != 1 || elegida != 3 {
		t.Fatalf("solo debería tener Sur: %v %d", ss, elegida)
	}
}

func TestSinRolesNoHaySesion(t *testing.T) {
	if _, _, _, err := ResolverAcceso(activas, nil, nil); err != ErrSinSesion {
		t.Fatalf("esperaba ErrSinSesion, obtuve %v", err)
	}
}

func TestValidarClave(t *testing.T) {
	if ValidarClave("corta") != ErrClaveCorta || ValidarClave("ñandubay") != nil {
		t.Fatal("validación de largo incorrecta")
	}
}
