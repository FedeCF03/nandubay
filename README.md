# Ñandubay

Sistema de gestión para carpinterías: stock de placas y herrajes, pedidos con
sus muebles, optimizador de cortes de melamina, usuarios con permisos y
estadísticas de ventas. Pensado para varias carpinterías, cada una con sus
datos separados y con varias sucursales.

- `backend/`: API en Go (chi + pgx) con PostgreSQL, organizada con Repository Pattern
  (handlers → servicios → repositorios, con los modelos aparte). Migraciones en `backend/migrations` (goose).
- `frontend/`: React + TypeScript + Vite, shadcn/ui (Tailwind + Radix), TanStack Query y wouter como router. Cada pantalla se carga por separado.
- `deploy/` y `docker-compose.yml`: despliegue en un VPS con Docker.
- `docs/diseno.md`: arquitectura, capas y modelo de datos.

## Desarrollo

Requisitos: Go 1.26, Node 22 y un PostgreSQL 16 o superior.

```sh
# Base de datos (una vez). El usuario NO debe ser superusuario, si no se saltea RLS.
createuser nandubay -P
createdb nandubay -O nandubay

# API en http://localhost:8080 (aplica las migraciones al arrancar).
# COOKIE_SEGURA=0 permite la cookie de sesión sin HTTPS, solo en desarrollo.
cd backend
export DATABASE_URL=postgres://nandubay:clave@localhost:5432/nandubay
MIGRAR=1 COOKIE_SEGURA=0 go run ./cmd/api

# Crear una carpintería con su usuario admin (pide la contraseña)
go run ./cmd/nandubay-admin crear-carpinteria -nombre "Carpintería López" \
  -admin-nombre "Juan López" -admin-email juan@ejemplo.com

# Opcional: datos de prueba (placas, stock en dos depósitos, retazos, clientes,
# ventas, presupuestos de muebles y cortes) en la carpintería de ese usuario
go run ./cmd/nandubay-admin cargar-demo -email juan@ejemplo.com

# Frontend en http://localhost:5173 (redirige /api a la API)
cd frontend
npm install
npm run dev
```

Tests: `cd backend && go test ./...`. Los de login y permisos necesitan una base
vacía y un usuario sin superusuario (si no, Row Level Security no se aplica):

```sh
createuser nandubay_test -P && createdb nandubay_test -O nandubay_test
NANDUBAY_TEST_DB=postgres://nandubay_test:clave@localhost:5432/nandubay_test go test ./...
```

## Producción (VPS)

```sh
cp .env.example .env   # completar dominio y claves
docker compose up -d --build
```

Levanta Postgres, la API, Caddy (HTTPS automático + frontend) y un backup diario
de la base en `./backups`. Para dar de alta una carpintería:

```sh
docker compose exec api /nandubay-admin crear-carpinteria -nombre "Carpintería López" \
  -admin-nombre "Juan López" -admin-email juan@ejemplo.com
```
