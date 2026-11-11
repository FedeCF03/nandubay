package acceso

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/FedeCF03/nandubay/backend/internal/web"
	"github.com/go-chi/chi/v5"
)

// Handler expone login, sesión y permisos por HTTP.
type Handler struct {
	Servicio     *Servicio
	Limitador    *web.Limitador
	CookieSegura bool
}

const cookieSesion = "nandubay_sesion"

type claveCtx struct{}

// UsuarioDe devuelve el usuario autenticado de la petición.
func UsuarioDe(ctx context.Context) *Usuario {
	u, _ := ctx.Value(claveCtx{}).(*Usuario)
	return u
}

// RequiereSesion corta con 401 si no hay una sesión válida.
func (h *Handler) RequiereSesion(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := h.Servicio.Autenticar(r.Context(), tokenDe(r))
		if errors.Is(err, ErrSinSesion) {
			web.ResponderError(w, http.StatusUnauthorized, "tenés que iniciar sesión")
			return
		}
		if err != nil {
			web.ErrorInterno(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claveCtx{}, u)))
	})
}

// RequierePermiso corta con 403 si el usuario no tiene el permiso en la sucursal actual.
func RequierePermiso(codigo string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if u := UsuarioDe(r.Context()); u == nil || !u.Permisos.Tiene(codigo) {
				web.ResponderError(w, http.StatusForbidden, "no tenés permiso para hacer esto")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type pedidoLogin struct {
	Email string `json:"email"`
	Clave string `json:"clave"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var p pedidoLogin
	if !web.LeerJSON(w, r, &p) {
		return
	}
	ip := ipDe(r)
	if !h.Limitador.Permitir(ip) {
		web.ResponderError(w, http.StatusTooManyRequests, "demasiados intentos, esperá unos minutos")
		return
	}
	token, err := h.Servicio.Login(r.Context(), p.Email, p.Clave, ip, r.UserAgent())
	if errors.Is(err, ErrCredenciales) {
		web.ResponderError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if err != nil {
		web.ErrorInterno(w, r, err)
		return
	}
	h.Limitador.Limpiar(ip)
	http.SetCookie(w, &http.Cookie{
		Name: cookieSesion, Value: token, Path: "/",
		MaxAge: int(DuracionSesion.Seconds()), HttpOnly: true,
		Secure: h.CookieSegura, SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if t := tokenDe(r); t != "" {
		if err := h.Servicio.Logout(r.Context(), t); err != nil {
			web.ErrorInterno(w, r, err)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name: cookieSesion, Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
		Secure: h.CookieSegura, SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

// usuarioJSON es cómo se muestra el usuario hacia afuera.
type usuarioJSON struct {
	ID            int64      `json:"id"`
	Nombre        string     `json:"nombre"`
	Email         string     `json:"email"`
	CarpinteriaID int64      `json:"carpinteria_id"`
	Carpinteria   string     `json:"carpinteria"`
	SucursalID    int64      `json:"sucursal_id"`
	Sucursales    []Sucursal `json:"sucursales"`
	Permisos      []string   `json:"permisos"`
}

func (h *Handler) yo(w http.ResponseWriter, r *http.Request) {
	u := UsuarioDe(r.Context())
	web.ResponderJSON(w, http.StatusOK, usuarioJSON{
		u.ID, u.Nombre, u.Email, u.CarpinteriaID, u.Carpinteria, u.SucursalID, u.Sucursales, u.Permisos.Lista(),
	})
}

func (h *Handler) elegirSucursal(w http.ResponseWriter, r *http.Request) {
	var p struct {
		SucursalID int64 `json:"sucursal_id"`
	}
	if !web.LeerJSON(w, r, &p) {
		return
	}
	err := h.Servicio.ElegirSucursal(r.Context(), UsuarioDe(r.Context()), tokenDe(r), p.SucursalID)
	if errors.Is(err, ErrSinSucursal) {
		web.ResponderError(w, http.StatusForbidden, err.Error())
		return
	}
	if err != nil {
		web.ErrorInterno(w, r, err)
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

// RutasPublicas son las que no necesitan sesión.
func (h *Handler) RutasPublicas(r chi.Router) {
	r.Post("/auth/login", h.login)
	r.Post("/auth/logout", h.logout)
}

// Rutas son las de la sesión ya iniciada.
func (h *Handler) Rutas(r chi.Router) {
	r.Get("/auth/yo", h.yo)
	r.Post("/auth/sucursal", h.elegirSucursal)
}
