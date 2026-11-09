package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/FedeCF03/nandubay/backend/internal/auth"
)

const cookieSesion = "nandubay_sesion"

type claveCtx struct{}

// UsuarioDe devuelve el usuario autenticado de la petición.
func UsuarioDe(ctx context.Context) *auth.Usuario {
	u, _ := ctx.Value(claveCtx{}).(*auth.Usuario)
	return u
}

// requiereSesion corta con 401 si no hay una sesión válida.
func (s *Servidor) requiereSesion(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var token string
		if c, err := r.Cookie(cookieSesion); err == nil {
			token = c.Value
		}
		u, err := s.Auth.Autenticar(r.Context(), token)
		if errors.Is(err, auth.ErrSinSesion) {
			responderError(w, http.StatusUnauthorized, "tenés que iniciar sesión")
			return
		}
		if err != nil {
			s.errorInterno(w, r, err)
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
	token, err := s.Auth.Login(r.Context(), p.Email, p.Clave, ip, r.UserAgent())
	if errors.Is(err, auth.ErrCredenciales) {
		responderError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if err != nil {
		s.errorInterno(w, r, err)
		return
	}
	s.Limitador.Limpiar(ip)
	http.SetCookie(w, &http.Cookie{
		Name: cookieSesion, Value: token, Path: "/",
		MaxAge: int(auth.DuracionSesion.Seconds()), HttpOnly: true,
		Secure: s.CookieSegura, SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Servidor) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieSesion); err == nil {
		if err := s.Auth.Logout(r.Context(), c.Value); err != nil {
			s.errorInterno(w, r, err)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name: cookieSesion, Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
		Secure: s.CookieSegura, SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

type respuestaYo struct {
	*auth.Usuario
	Permisos []string `json:"permisos"`
}

func (s *Servidor) yo(w http.ResponseWriter, r *http.Request) {
	u := UsuarioDe(r.Context())
	responderJSON(w, http.StatusOK, respuestaYo{u, u.Permisos.Lista()})
}

func (s *Servidor) elegirSucursal(w http.ResponseWriter, r *http.Request) {
	var p struct {
		SucursalID int64 `json:"sucursal_id"`
	}
	if !leerJSON(w, r, &p) {
		return
	}
	err := s.Auth.ElegirSucursal(r.Context(), UsuarioDe(r.Context()), p.SucursalID)
	if errors.Is(err, auth.ErrSinSucursal) {
		responderError(w, http.StatusForbidden, err.Error())
		return
	}
	if err != nil {
		s.errorInterno(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func leerJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		responderError(w, http.StatusBadRequest, "JSON inválido")
		return false
	}
	return true
}

// ipDe usa la IP que dejó middleware.RealIP (Caddy manda X-Forwarded-For).
func ipDe(r *http.Request) string {
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return h
	}
	return r.RemoteAddr
}

func (s *Servidor) errorInterno(w http.ResponseWriter, r *http.Request, err error) {
	slog.ErrorContext(r.Context(), "error interno", "ruta", r.URL.Path, "error", err)
	responderError(w, http.StatusInternalServerError, "algo salió mal, probá de nuevo")
}
