// Package web tiene lo que comparten los handler de todos los módulos: leer
// y responder JSON, errores, el id de la ruta y el control de Content-Type.
package web

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

func LeerJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		ResponderError(w, http.StatusBadRequest, "JSON inválido")
		return false
	}
	return true
}

func ResponderJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func ResponderError(w http.ResponseWriter, status int, msg string) {
	ResponderJSON(w, status, map[string]string{"error": msg})
}

func ErrorInterno(w http.ResponseWriter, r *http.Request, err error) {
	slog.ErrorContext(r.Context(), "error interno", "ruta", r.URL.Path, "error", err)
	ResponderError(w, http.StatusInternalServerError, "algo salió mal, probá de nuevo")
}

// IDDe lee el {id} de la ruta. Si no es válido responde 400 y devuelve false.
func IDDe(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		ResponderError(w, http.StatusBadRequest, "id inválido")
		return 0, false
	}
	return id, true
}

// SoloJSON exige Content-Type JSON en los pedidos que cambian datos. Un
// formulario de otro sitio no puede mandarlo sin permiso de CORS, así que
// junto con la cookie SameSite=Lax evita ataques CSRF.
func SoloJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				ResponderError(w, http.StatusUnsupportedMediaType, "se espera JSON")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
