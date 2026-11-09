# Ñandubay

Sistema de gestión para carpinterías: stock de placas y herrajes, pedidos con
sus muebles, optimizador de cortes de melamina, usuarios con permisos y
estadísticas de ventas. Pensado para varias carpinterías, cada una con sus
datos separados y con varias sucursales.

- `backend/`: API en Go (chi + pgx) con PostgreSQL. Migraciones en `backend/migrations` (goose).
- `frontend/`: React + TypeScript + Vite, TanStack Query y Tailwind.
- `deploy/` y `docker-compose.yml`: despliegue en un VPS con Docker.
- `docs/diseno.md`: arquitectura y modelo de datos.

## Desarrollo

Requisitos: Go 1.26, Node 22 y un PostgreSQL 16 o superior.

```sh
# Base de datos (una vez). El usuario NO debe ser superusuario, si no se saltea RLS.
createuser nandubay -P
createdb nandubay -O nandubay

# API en http://localhost:8080 (aplica las migraciones al arrancar)
cd backend
DATABASE_URL=postgres://nandubay:clave@localhost:5432/nandubay MIGRAR=1 go run ./cmd/api

# Frontend en http://localhost:5173 (redirige /api a la API)
cd frontend
npm install
npm run dev
```

Tests del optimizador: `cd backend && go test ./...`

## Producción (VPS)

```sh
cp .env.example .env   # completar dominio y claves
docker compose up -d --build
```

Levanta Postgres, la API, Caddy (HTTPS automático + frontend) y un backup diario
de la base en `./backups`.
