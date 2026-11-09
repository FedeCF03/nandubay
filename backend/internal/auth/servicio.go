// Package auth maneja el alta de carpinterías, el login, las sesiones y los permisos.
package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/FedeCF03/nandubay/backend/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const DuracionSesion = 30 * 24 * time.Hour

var (
	ErrCredenciales = errors.New("email o contraseña incorrectos")
	ErrSinSesion    = errors.New("no hay sesión")
	ErrEmailEnUso   = errors.New("ya hay un usuario con ese email")
	ErrSinSucursal  = errors.New("el usuario no tiene acceso a esa sucursal")
)

type Servicio struct {
	DB *pgxpool.Pool
}

// Sucursal disponible para el usuario.
type Sucursal struct {
	ID     int64  `json:"id"`
	Nombre string `json:"nombre"`
}

// Usuario es quien hizo la petición, con sus permisos en la sucursal elegida.
type Usuario struct {
	ID            int64      `json:"id"`
	Nombre        string     `json:"nombre"`
	Email         string     `json:"email"`
	CarpinteriaID int64      `json:"carpinteria_id"`
	Carpinteria   string     `json:"carpinteria"`
	SucursalID    int64      `json:"sucursal_id"`
	Sucursales    []Sucursal `json:"sucursales"`
	Permisos      Permisos   `json:"-"`
	tokenHash     string
}

// ---------------------------------------------------------------------------
// Alta de carpintería
// ---------------------------------------------------------------------------

type NuevaCarpinteria struct {
	Nombre      string
	Sucursal    string
	AdminNombre string
	AdminEmail  string
	AdminClave  string
}

// CrearCarpinteria da de alta una carpintería con una sucursal, un depósito,
// los roles base y su usuario administrador.
func (s *Servicio) CrearCarpinteria(ctx context.Context, n NuevaCarpinteria) (int64, error) {
	n.Nombre = strings.TrimSpace(n.Nombre)
	n.AdminEmail = strings.ToLower(strings.TrimSpace(n.AdminEmail))
	if n.Nombre == "" || n.AdminNombre == "" || !strings.Contains(n.AdminEmail, "@") {
		return 0, errors.New("faltan el nombre de la carpintería, el nombre o el email del admin")
	}
	if n.Sucursal == "" {
		n.Sucursal = "Casa central"
	}
	hash, err := HashClave(n.AdminClave)
	if err != nil {
		return 0, err
	}

	var id int64
	err = pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO carpinterias (nombre) VALUES ($1) RETURNING id`, n.Nombre).Scan(&id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT set_config('app.carpinteria_id', $1, true)`, fmt.Sprint(id)); err != nil {
			return err
		}
		var sucursalID, usuarioID, adminRol int64
		if err := tx.QueryRow(ctx, `INSERT INTO sucursales (carpinteria_id, nombre) VALUES ($1, $2) RETURNING id`,
			id, n.Sucursal).Scan(&sucursalID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO depositos (carpinteria_id, sucursal_id, nombre) VALUES ($1, $2, 'Depósito principal')`,
			id, sucursalID); err != nil {
			return err
		}
		for _, r := range rolesBase {
			var rolID int64
			if err := tx.QueryRow(ctx, `INSERT INTO roles (carpinteria_id, nombre, es_sistema) VALUES ($1, $2, true) RETURNING id`,
				id, r.Nombre).Scan(&rolID); err != nil {
				return err
			}
			if r.Permisos == nil {
				adminRol = rolID
				_, err = tx.Exec(ctx, `INSERT INTO rol_permisos (carpinteria_id, rol_id, permiso) SELECT $1, $2, codigo FROM permisos`, id, rolID)
			} else {
				_, err = tx.Exec(ctx, `INSERT INTO rol_permisos (carpinteria_id, rol_id, permiso) SELECT $1, $2, unnest($3::text[])`, id, rolID, r.Permisos)
			}
			if err != nil {
				return err
			}
		}
		if err := tx.QueryRow(ctx, `INSERT INTO usuarios (carpinteria_id, email, nombre, password_hash) VALUES ($1, $2, $3, $4) RETURNING id`,
			id, n.AdminEmail, n.AdminNombre, hash).Scan(&usuarioID); err != nil {
			return traducirError(err)
		}
		_, err := tx.Exec(ctx, `INSERT INTO usuario_roles (carpinteria_id, usuario_id, rol_id, sucursal_id) VALUES ($1, $2, $3, NULL)`,
			id, usuarioID, adminRol)
		return err
	})
	return id, err
}

func traducirError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
		(pgErr.ConstraintName == "usuarios_email_unico" || pgErr.ConstraintName == "cuentas_login_pkey") {
		return ErrEmailEnUso
	}
	return err
}

// ---------------------------------------------------------------------------
// Login y sesiones
// ---------------------------------------------------------------------------

// Login valida email y contraseña y abre una sesión. Devuelve el token para la cookie.
func (s *Servicio) Login(ctx context.Context, email, clave, ip, agente string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var usuarioID, carpinteriaID int64
	err := s.DB.QueryRow(ctx, `
		SELECT cl.usuario_id, cl.carpinteria_id FROM cuentas_login cl
		JOIN carpinterias c ON c.id = cl.carpinteria_id AND c.activa
		WHERE cl.email = $1`, email).Scan(&usuarioID, &carpinteriaID)
	if errors.Is(err, pgx.ErrNoRows) {
		claveCorrecta("", clave)
		return "", ErrCredenciales
	}
	if err != nil {
		return "", err
	}

	var hash string
	var activo bool
	err = db.EnCarpinteria(ctx, s.DB, carpinteriaID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT password_hash, activo FROM usuarios WHERE id = $1`, usuarioID).Scan(&hash, &activo)
	})
	if err != nil {
		return "", err
	}
	if !claveCorrecta(hash, clave) || !activo {
		return "", ErrCredenciales
	}

	_ = db.EnCarpinteria(ctx, s.DB, carpinteriaID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE usuarios SET ultimo_ingreso = now() WHERE id = $1`, usuarioID)
		return err
	})

	token, tokenHash, err := nuevoToken()
	if err != nil {
		return "", err
	}
	_, err = s.DB.Exec(ctx, `
		INSERT INTO sesiones (token_hash, usuario_id, carpinteria_id, expira_en, ip, agente)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		tokenHash, usuarioID, carpinteriaID, time.Now().Add(DuracionSesion), ip, recortar(agente, 300))
	if err != nil {
		return "", err
	}
	// De paso, limpiar sesiones vencidas.
	_, _ = s.DB.Exec(ctx, `DELETE FROM sesiones WHERE expira_en < now()`)
	return token, nil
}

func (s *Servicio) Logout(ctx context.Context, token string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM sesiones WHERE token_hash = $1`, hashToken(token))
	return err
}

// Autenticar devuelve el usuario dueño del token, con sus permisos en la
// sucursal elegida en la sesión.
func (s *Servicio) Autenticar(ctx context.Context, token string) (*Usuario, error) {
	if token == "" {
		return nil, ErrSinSesion
	}
	u := &Usuario{tokenHash: hashToken(token)}
	var sucursalID *int64
	err := s.DB.QueryRow(ctx, `
		UPDATE sesiones SET usada_en = now()
		WHERE token_hash = $1 AND expira_en > now()
		RETURNING usuario_id, carpinteria_id, sucursal_id`, u.tokenHash).Scan(&u.ID, &u.CarpinteriaID, &sucursalID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSinSesion
	}
	if err != nil {
		return nil, err
	}
	err = db.EnCarpinteria(ctx, s.DB, u.CarpinteriaID, func(tx pgx.Tx) error {
		var activo bool
		err := tx.QueryRow(ctx, `
			SELECT u.nombre, u.email, u.activo, c.nombre
			FROM usuarios u JOIN carpinterias c ON c.id = u.carpinteria_id AND c.activa
			WHERE u.id = $1`, u.ID).Scan(&u.Nombre, &u.Email, &activo, &u.Carpinteria)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !activo) {
			return ErrSinSesion
		}
		if err != nil {
			return err
		}
		if u.Sucursales, err = sucursalesDe(ctx, tx, u.ID); err != nil {
			return err
		}
		if len(u.Sucursales) == 0 {
			return ErrSinSesion
		}
		u.SucursalID = u.Sucursales[0].ID
		if sucursalID != nil && contiene(u.Sucursales, *sucursalID) {
			u.SucursalID = *sucursalID
		}
		u.Permisos, err = permisosDe(ctx, tx, u.ID, u.SucursalID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return u, nil
}

// ElegirSucursal cambia la sucursal de la sesión.
func (s *Servicio) ElegirSucursal(ctx context.Context, u *Usuario, sucursalID int64) error {
	if !contiene(u.Sucursales, sucursalID) {
		return ErrSinSucursal
	}
	_, err := s.DB.Exec(ctx, `UPDATE sesiones SET sucursal_id = $1 WHERE token_hash = $2`, sucursalID, u.tokenHash)
	return err
}

// sucursalesDe devuelve las sucursales donde el usuario tiene algún rol.
// Un rol sin sucursal vale para todas.
func sucursalesDe(ctx context.Context, tx pgx.Tx, usuarioID int64) ([]Sucursal, error) {
	rows, err := tx.Query(ctx, `
		SELECT s.id, s.nombre FROM sucursales s
		WHERE s.activa AND EXISTS (
			SELECT 1 FROM usuario_roles ur
			WHERE ur.usuario_id = $1 AND (ur.sucursal_id IS NULL OR ur.sucursal_id = s.id))
		ORDER BY s.nombre`, usuarioID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Sucursal])
}

func permisosDe(ctx context.Context, tx pgx.Tx, usuarioID, sucursalID int64) (Permisos, error) {
	rows, err := tx.Query(ctx, `
		SELECT DISTINCT rp.permiso FROM usuario_roles ur
		JOIN rol_permisos rp ON rp.rol_id = ur.rol_id
		WHERE ur.usuario_id = $1 AND (ur.sucursal_id IS NULL OR ur.sucursal_id = $2)`, usuarioID, sucursalID)
	if err != nil {
		return nil, err
	}
	lista, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	p := Permisos{}
	for _, c := range lista {
		p[c] = true
	}
	return p, nil
}

func contiene(ss []Sucursal, id int64) bool {
	for _, s := range ss {
		if s.ID == id {
			return true
		}
	}
	return false
}

func recortar(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
