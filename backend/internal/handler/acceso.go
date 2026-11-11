package handler

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/FedeCF03/nandubay/backend/internal/model/acceso"
	"github.com/FedeCF03/nandubay/backend/internal/service"
)

const cookieSesion = "nandubay_sesion"

type claveCtx struct{}

// UsuarioDe devuelve el usuario autenticado de la petición.
func UsuarioDe(ctx context.Context) *acceso.Usuario {
	u, _ := ctx.Value(claveCtx{}).(*acceso.Usuario)
	return u
}

// requiereSesion corta con 401 si no hay una sesión válida.
func (s *Servidor) requiereSesion(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := s.Acceso.Autenticar(r.Context(), tokenDe(r))
		if errors.Is(err, acceso.ErrSinSesion) {
			responderError(w, http.StatusUnauthorized, "tenés que iniciar sesión")
			return
		}
		if err != nil {
			errorInterno(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claveCtx{}, u)))
	})
}

// requierePermiso corta con 403 si el usuario no tiene el permiso en la sucursal actual.
func requierePermiso(codigo string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if u := UsuarioDe(r.Context()); u == nil || !u.Permisos.Tiene(codigo) {
				responderError(w, http.StatusForbidden, "no tenés permiso para hacer esto")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// soloJSON exige Content-Type JSON en los pedidos que cambian datos. Un
// formulario de otro sitio no puede mandarlo sin permiso de CORS, así que
// junto con la cookie SameSite=Lax evita ataques CSRF.
func soloJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				responderError(w, http.StatusUnsupportedMediaType, "se espera JSON")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

type pedidoLogin struct {
	Email string `json:"email"`
	Clave string `json:"clave"`
}

func (s *Servidor) login(w http.ResponseWriter, r *http.Request) {
	var p pedidoLogin
	if !leerJSON(w, r, &p) {
		return
	}
	ip := ipDe(r)
	if !s.Limitador.Permitir(ip) {
		responderError(w, http.StatusTooManyRequests, "demasiados intentos, esperá unos minutos")
		return
	}
	token, err := s.Acceso.Login(r.Context(), p.Email, p.Clave, ip, r.UserAgent())
	if errors.Is(err, acceso.ErrCredenciales) {
		responderError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if err != nil {
		errorInterno(w, r, err)
		return
	}
	s.Limitador.Limpiar(ip)
	http.SetCookie(w, &http.Cookie{
		Name: cookieSesion, Value: token, Path: "/",
		MaxAge: int(service.DuracionSesion.Seconds()), HttpOnly: true,
		Secure: s.CookieSegura, SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Servidor) logout(w http.ResponseWriter, r *http.Request) {
	if t := tokenDe(r); t != "" {
		if err := s.Acceso.Logout(r.Context(), t); err != nil {
			errorInterno(w, r, err)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name: cookieSesion, Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
		Secure: s.CookieSegura, SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

// usuarioJSON es cómo se muestra el usuario hacia afuera.
type usuarioJSON struct {
	ID            int64             `json:"id"`
	Nombre        string            `json:"nombre"`
	Email         string            `json:"email"`
	CarpinteriaID int64             `json:"carpinteria_id"`
	Carpinteria   string            `json:"carpinteria"`
	SucursalID    int64             `json:"sucursal_id"`
	Sucursales    []acceso.Sucursal `json:"sucursales"`
	Permisos      []string          `json:"permisos"`
}

func (s *Servidor) yo(w http.ResponseWriter, r *http.Request) {
	u := UsuarioDe(r.Context())
	responderJSON(w, http.StatusOK, usuarioJSON{
		u.ID, u.Nombre, u.Email, u.CarpinteriaID, u.Carpinteria, u.SucursalID, u.Sucursales, u.Permisos.Lista(),
	})
}

func (s *Servidor) elegirSucursal(w http.ResponseWriter, r *http.Request) {
	var p struct {
		SucursalID int64 `json:"sucursal_id"`
	}
	if !leerJSON(w, r, &p) {
		return
	}
	err := s.Acceso.ElegirSucursal(r.Context(), UsuarioDe(r.Context()), tokenDe(r), p.SucursalID)
	if errors.Is(err, acceso.ErrSinSucursal) {
		responderError(w, http.StatusForbidden, err.Error())
		return
	}
	if err != nil {
		errorInterno(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func tokenDe(r *http.Request) string {
	if c, err := r.Cookie(cookieSesion); err == nil {
		return c.Value
	}
	return ""
}

// ipDe usa la IP que dejó middleware.RealIP (Caddy manda X-Forwarded-For).
func ipDe(r *http.Request) string {
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return h
	}
	return r.RemoteAddr
}
