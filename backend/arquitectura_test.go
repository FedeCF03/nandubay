package backend_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Regla de dependencias de clean architecture: las capas internas no
// importan a las externas. Si este test falla, hay un import en la capa
// equivocada.
const modulo = "github.com/FedeCF03/nandubay/backend/internal/"

var reglas = []struct {
	capa      string
	permitido []string // imports internos permitidos
	externos  bool     // si puede usar librerías de terceros
}{
	{"internal/dominio", []string{"dominio/"}, false},
	{"internal/aplicacion", []string{"dominio/", "aplicacion"}, false},
}

func TestReglaDeDependencias(t *testing.T) {
	for _, r := range reglas {
		err := filepath.WalkDir(r.capa, func(ruta string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(ruta, ".go") {
				return err
			}
			f, err := parser.ParseFile(token.NewFileSet(), ruta, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range f.Imports {
				p, _ := strconv.Unquote(imp.Path.Value)
				switch {
				case strings.HasPrefix(p, modulo):
					ok := false
					for _, perm := range r.permitido {
						ok = ok || strings.HasPrefix(strings.TrimPrefix(p, modulo), perm)
					}
					if !ok {
						t.Errorf("%s importa %s, que es de una capa externa", ruta, p)
					}
				case strings.Contains(strings.SplitN(p, "/", 2)[0], ".") && !r.externos:
					t.Errorf("%s importa la librería externa %s; la capa %s no debe depender de librerías", ruta, p, r.capa)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
