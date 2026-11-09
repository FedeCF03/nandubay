// Package migrations contiene las migraciones SQL (formato goose) embebidas en el binario.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
