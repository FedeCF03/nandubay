package backend_test

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Cada módulo (internal/acceso, stock, cortes, ventas, muebles) es una carpeta
// con su modelo, su servicio, su Store (interfaz), su PostgresStore y su
// handler. Este test controla que cada archivo use solo lo que le corresponde:
//
//   - modelo, servicio y store.go: nada de base de datos ni de HTTP. El
//     servicio habla con la base solo a través de la interfaz Store.
//   - postgres_*.go: la base, pero nada de HTTP.
//   - handler.go: HTTP, pero nada de base de datos.
const modulo = "github.com/FedeCF03/nandubay/backend/internal/"

var (
	deBase = []string{"github.com/jackc/pgx", modulo + "db"}
	deHTTP = []string{"github.com/go-chi/chi", modulo + "web"}
)

func TestImportsPorArchivo(t *testing.T) {
	archivos, err := filepath.Glob("internal/*/*.go")
	if err != nil {
		t.Fatal(err)
	}
	modulos := map[string]bool{"acceso": true, "stock": true, "cortes": true, "ventas": true, "muebles": true}
	for _, ruta := range archivos {
		nombre := filepath.Base(ruta)
		if !modulos[filepath.Base(filepath.Dir(ruta))] || strings.HasSuffix(nombre, "_test.go") {
			continue
		}
		var prohibidos []string
		switch {
		case strings.HasPrefix(nombre, "postgres_"):
			prohibidos = deHTTP
		case nombre == "handler.go":
			prohibidos = deBase
		default:
			prohibidos = append(append([]string{}, deBase...), deHTTP...)
		}
		f, err := parser.ParseFile(token.NewFileSet(), ruta, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			for _, x := range prohibidos {
				if strings.HasPrefix(p, x) {
					t.Errorf("%s importa %s, que no le corresponde", ruta, p)
				}
			}
		}
	}
}
