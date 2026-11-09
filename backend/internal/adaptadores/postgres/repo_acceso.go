package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/FedeCF03/nandubay/backend/internal/aplicacion"
	"github.com/FedeCF03/nandubay/backend/internal/dominio/acceso"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepoAcceso implementa aplicacion.RepositorioAcceso.
type RepoAcceso struct {
	DB *pgxpool.Pool
}

var _ aplicacion.RepositorioAcceso = (*RepoAcceso)(nil)

func noEncontrado(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return acceso.ErrNoEncontrado
	}
	return err
}

func (r *RepoAcceso) BuscarCuenta(ctx context.Context, email string) (aplicacion.CuentaLogin, error) {
	var c aplicacion.CuentaLogin
	// cuentas_login no tiene RLS: todavía no se sabe la carpintería.
	err := r.DB.QueryRow(ctx, `
		SELECT cl.usuario_id, cl.carpinteria_id FROM cuentas_login cl
		JOIN carpinterias ca ON ca.id = cl.carpinteria_id AND ca.activa
		WHERE cl.email = $1`, email).Scan(&c.UsuarioID, &c.CarpinteriaID)
	if err != nil {
		return c, noEncontrado(err)
	}
	err = EnCarpinteria(ctx, r.DB, c.CarpinteriaID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT password_hash, activo FROM usuarios WHERE id = $1`, c.UsuarioID).
			Scan(&c.HashClave, &c.Activo)
	})
	return c, noEncontrado(err)
}

func (r *RepoAcceso) RegistrarIngreso(ctx context.Context, carpinteriaID, usuarioID int64) error {
	return EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE usuarios SET ultimo_ingreso = now() WHERE id = $1`, usuarioID)
		return err
	})
}

func (r *RepoAcceso) CargarAcceso(ctx context.Context, carpinteriaID, usuarioID int64) (aplicacion.DatosAcceso, error) {
	var d aplicacion.DatosAcceso
	err := EnCarpinteria(ctx, r.DB, carpinteriaID, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT u.nombre, u.email, u.activo, c.nombre
			FROM usuarios u JOIN carpinterias c ON c.id = u.carpinteria_id AND c.activa
			WHERE u.id = $1`, usuarioID).Scan(&d.Nombre, &d.Email, &d.Activo, &d.Carpinteria)
		if err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT id, nombre FROM sucursales WHERE activa ORDER BY nombre`)
		if err != nil {
			return err
		}
		if d.SucursalesActivas, err = pgx.CollectRows(rows, pgx.RowToStructByPos[acceso.Sucursal]); err != nil {
			return err
		}
		rows, err = tx.Query(ctx, `
			SELECT ur.sucursal_id, rp.permiso FROM usuario_roles ur
			JOIN rol_permisos rp ON rp.rol_id = ur.rol_id
			WHERE ur.usuario_id = $1`, usuarioID)
		if err != nil {
			return err
		}
		d.Asignaciones, err = pgx.CollectRows(rows, pgx.RowToStructByPos[acceso.Asignacion])
		return err
	})
	return d, noEncontrado(err)
}

func (r *RepoAcceso) CrearCarpinteria(ctx context.Context, a aplicacion.AltaCarpinteria) (int64, error) {
	var id int64
	err := pgx.BeginFunc(ctx, r.DB, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO carpinterias (nombre) VALUES ($1) RETURNING id`, a.Nombre).Scan(&id); err != nil {
			return err
		}
		if err := fijarCarpinteria(ctx, tx, id); err != nil {
			return err
		}
		var sucursalID, usuarioID, rolAdmin int64
		if err := tx.QueryRow(ctx, `INSERT INTO sucursales (carpinteria_id, nombre) VALUES ($1, $2) RETURNING id`,
			id, a.Sucursal).Scan(&sucursalID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO depositos (carpinteria_id, sucursal_id, nombre) VALUES ($1, $2, $3)`,
			id, sucursalID, a.Deposito); err != nil {
			return err
		}
		for _, rol := range a.Roles {
			var rolID int64
			if err := tx.QueryRow(ctx, `INSERT INTO roles (carpinteria_id, nombre, es_sistema) VALUES ($1, $2, true) RETURNING id`,
				id, rol.Nombre).Scan(&rolID); err != nil {
				return err
			}
			var err error
			if rol.Todos {
				_, err = tx.Exec(ctx, `INSERT INTO rol_permisos (carpinteria_id, rol_id, permiso) SELECT $1, $2, codigo FROM permisos`, id, rolID)
			} else {
				_, err = tx.Exec(ctx, `INSERT INTO rol_permisos (carpinteria_id, rol_id, permiso) SELECT $1, $2, unnest($3::text[])`, id, rolID, rol.Permisos)
			}
			if err != nil {
				return err
			}
			if rol.Nombre == a.RolAdmin {
				rolAdmin = rolID
			}
		}
		if err := tx.QueryRow(ctx, `INSERT INTO usuarios (carpinteria_id, email, nombre, password_hash) VALUES ($1, $2, $3, $4) RETURNING id`,
			id, a.AdminEmail, a.AdminNombre, a.AdminHashClave).Scan(&usuarioID); err != nil {
			return traducirError(err)
		}
		_, err := tx.Exec(ctx, `INSERT INTO usuario_roles (carpinteria_id, usuario_id, rol_id, sucursal_id) VALUES ($1, $2, $3, NULL)`,
			id, usuarioID, rolAdmin)
		return err
	})
	return id, err
}

func traducirError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
		(pgErr.ConstraintName == "usuarios_email_unico" || pgErr.ConstraintName == "cuentas_login_pkey") {
		return acceso.ErrEmailEnUso
	}
	return err
}

// Las sesiones no tienen RLS: se buscan por token antes de saber la carpintería.

func (r *RepoAcceso) GuardarSesion(ctx context.Context, s aplicacion.Sesion) error {
	_, err := r.DB.Exec(ctx, `
		INSERT INTO sesiones (token_hash, usuario_id, carpinteria_id, sucursal_id, expira_en, ip, agente)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		s.TokenHash, s.UsuarioID, s.CarpinteriaID, s.SucursalID, s.ExpiraEn, s.IP, s.Agente)
	return err
}

func (r *RepoAcceso) UsarSesion(ctx context.Context, tokenHash string, ahora time.Time) (aplicacion.Sesion, error) {
	s := aplicacion.Sesion{TokenHash: tokenHash}
	err := r.DB.QueryRow(ctx, `
		UPDATE sesiones SET usada_en = $2
		WHERE token_hash = $1 AND expira_en > $2
		RETURNING usuario_id, carpinteria_id, sucursal_id, expira_en`, tokenHash, ahora).
		Scan(&s.UsuarioID, &s.CarpinteriaID, &s.SucursalID, &s.ExpiraEn)
	return s, noEncontrado(err)
}

func (r *RepoAcceso) CambiarSucursalSesion(ctx context.Context, tokenHash string, sucursalID int64) error {
	_, err := r.DB.Exec(ctx, `UPDATE sesiones SET sucursal_id = $1 WHERE token_hash = $2`, sucursalID, tokenHash)
	return err
}

func (r *RepoAcceso) BorrarSesion(ctx context.Context, tokenHash string) error {
	_, err := r.DB.Exec(ctx, `DELETE FROM sesiones WHERE token_hash = $1`, tokenHash)
	return err
}

func (r *RepoAcceso) BorrarSesionesVencidas(ctx context.Context, ahora time.Time) error {
	_, err := r.DB.Exec(ctx, `DELETE FROM sesiones WHERE expira_en < $1`, ahora)
	return err
}
