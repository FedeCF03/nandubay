#!/bin/sh
# Crea el usuario de la aplicación sin permisos de superusuario.
# Importante: un superusuario se saltea Row Level Security, por eso la API
# nunca tiene que conectarse como "postgres".
set -e
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<SQL
CREATE ROLE nandubay LOGIN PASSWORD '${APP_DB_PASSWORD}' NOSUPERUSER NOBYPASSRLS;
CREATE DATABASE nandubay OWNER nandubay;
SQL
