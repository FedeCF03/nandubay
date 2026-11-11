// nandubay-admin es la herramienta de línea de comandos para tareas de
// administración de la plataforma.
//
//	nandubay-admin crear-carpinteria -nombre "Carpintería López" \
//	    -admin-nombre "Juan López" -admin-email juan@ejemplo.com
//	nandubay-admin cargar-demo -email juan@ejemplo.com
//
// La contraseña del admin se toma de ADMIN_CLAVE o se pide por consola.
// cargar-demo carga datos de prueba en la carpintería de ese usuario.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/FedeCF03/nandubay/backend/internal/repositorios"
	"github.com/FedeCF03/nandubay/backend/internal/seguridad"
	"github.com/FedeCF03/nandubay/backend/internal/servicios"
)

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "cargar-demo" {
		if err := cargarDemo(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) < 2 || os.Args[1] != "crear-carpinteria" {
		fmt.Fprintln(os.Stderr, "uso: nandubay-admin crear-carpinteria -nombre ... -admin-nombre ... -admin-email ...")
		fmt.Fprintln(os.Stderr, "     nandubay-admin cargar-demo -email ...")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("crear-carpinteria", flag.ExitOnError)
	nombre := fs.String("nombre", "", "nombre de la carpintería")
	sucursal := fs.String("sucursal", "Casa central", "nombre de la primera sucursal")
	adminNombre := fs.String("admin-nombre", "", "nombre del usuario administrador")
	adminEmail := fs.String("admin-email", "", "email del usuario administrador")
	_ = fs.Parse(os.Args[2:])

	clave := os.Getenv("ADMIN_CLAVE")
	if clave == "" {
		fmt.Print("Contraseña del admin (mínimo 8 caracteres): ")
		linea, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		clave = strings.TrimRight(linea, "\r\n")
	}

	ctx := context.Background()
	pool, err := repositorios.Conectar(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := repositorios.Migrar(pool); err != nil {
		fmt.Fprintln(os.Stderr, "error en migraciones:", err)
		os.Exit(1)
	}
	casos := &servicios.ServicioAcceso{
		Repo: &repositorios.RepoAcceso{DB: pool}, Hasher: &seguridad.Bcrypt{}, Tokens: seguridad.Tokens{},
	}
	id, err := casos.CrearCarpinteria(ctx, servicios.NuevaCarpinteria{
		Nombre: *nombre, Sucursal: *sucursal, AdminNombre: *adminNombre, AdminEmail: *adminEmail, AdminClave: clave,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Printf("Carpintería creada (id %d). Ya podés entrar con %s.\n", id, *adminEmail)
}
