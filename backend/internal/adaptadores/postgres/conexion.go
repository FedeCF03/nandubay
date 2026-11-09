// Package postgres implementa los repositorios de la capa de aplicación con
// PostgreSQL, y maneja la conexión, las migraciones y el aislamiento por carpintería.
package postgres

import (
	"context"
	"fmt"
	"strconv"

	"github.com/FedeCF03/nandubay/backend/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func Conectar(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("no se pudo conectar a Postgres: %w", err)
	}
	return pool, nil
}

// Migrar aplica las migraciones pendientes.
func Migrar(pool *pgxpool.Pool) error {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(sqlDB, ".")
}

// EnCarpinteria ejecuta fn dentro de una transacción en la que Row Level
// Security solo deja ver y escribir filas de la carpintería indicada.
// Todo acceso a datos de una carpintería tiene que pasar por acá.
func EnCarpinteria(ctx context.Context, pool *pgxpool.Pool, carpinteriaID int64, fn func(pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if err := fijarCarpinteria(ctx, tx, carpinteriaID); err != nil {
			return err
		}
		return fn(tx)
	})
}

func fijarCarpinteria(ctx context.Context, tx pgx.Tx, carpinteriaID int64) error {
	_, err := tx.Exec(ctx, "SELECT set_config('app.carpinteria_id', $1, true)", strconv.FormatInt(carpinteriaID, 10))
	return err
}
