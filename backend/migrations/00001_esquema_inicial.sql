-- +goose Up
-- Esquema inicial de nandubay.
--
-- Multi-carpintería: cada tabla de datos tiene carpinteria_id y está protegida
-- con Row Level Security. La API, en cada transacción, hace
--   SELECT set_config('app.carpinteria_id', '<id>', true);
-- y Postgres filtra solo las filas de esa carpintería.
-- Las medidas están en milímetros (enteros) y las cantidades de canto en metros.

-- +goose StatementBegin
CREATE FUNCTION app_carpinteria_id() RETURNS bigint
LANGUAGE sql STABLE AS $$
  SELECT NULLIF(current_setting('app.carpinteria_id', true), '')::bigint
$$;
-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- Carpinterías, sucursales y depósitos
-- ---------------------------------------------------------------------------

CREATE TABLE carpinterias (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nombre      text NOT NULL,
    cuit        text,
    activa      boolean NOT NULL DEFAULT true,
    creado_en   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sucursales (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id  bigint NOT NULL REFERENCES carpinterias(id),
    nombre          text NOT NULL,
    direccion       text,
    telefono        text,
    activa          boolean NOT NULL DEFAULT true,
    UNIQUE (carpinteria_id, nombre)
);

CREATE TABLE depositos (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id  bigint NOT NULL REFERENCES carpinterias(id),
    sucursal_id     bigint NOT NULL REFERENCES sucursales(id),
    nombre          text NOT NULL,
    activo          boolean NOT NULL DEFAULT true,
    UNIQUE (carpinteria_id, nombre)
);

-- ---------------------------------------------------------------------------
-- Usuarios, roles y permisos
-- ---------------------------------------------------------------------------

-- Catálogo global de permisos (no depende de la carpintería).
CREATE TABLE permisos (
    codigo       text PRIMARY KEY,          -- ej: 'stock.editar'
    descripcion  text NOT NULL
);

CREATE TABLE usuarios (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id  bigint NOT NULL REFERENCES carpinterias(id),
    email           text NOT NULL,
    nombre          text NOT NULL,
    password_hash   text NOT NULL,
    activo          boolean NOT NULL DEFAULT true,
    creado_en       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (carpinteria_id, email)
);

-- Roles armables a medida por cada carpintería.
CREATE TABLE roles (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id  bigint NOT NULL REFERENCES carpinterias(id),
    nombre          text NOT NULL,
    es_sistema      boolean NOT NULL DEFAULT false,  -- admin, vendedor, operario, deposito
    UNIQUE (carpinteria_id, nombre)
);

CREATE TABLE rol_permisos (
    carpinteria_id  bigint NOT NULL REFERENCES carpinterias(id),
    rol_id          bigint NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permiso         text   NOT NULL REFERENCES permisos(codigo),
    PRIMARY KEY (rol_id, permiso)
);

-- Un usuario puede tener un rol distinto en cada sucursal.
-- sucursal_id NULL = el rol aplica a todas las sucursales.
CREATE TABLE usuario_roles (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id  bigint NOT NULL REFERENCES carpinterias(id),
    usuario_id      bigint NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    rol_id          bigint NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    sucursal_id     bigint REFERENCES sucursales(id),
    UNIQUE NULLS NOT DISTINCT (usuario_id, rol_id, sucursal_id)
);

-- ---------------------------------------------------------------------------
-- Clientes y proveedores
-- ---------------------------------------------------------------------------

CREATE TABLE clientes (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id  bigint NOT NULL REFERENCES carpinterias(id),
    nombre          text NOT NULL,
    telefono        text,
    email           text,
    direccion       text,
    notas           text,
    creado_en       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE proveedores (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id  bigint NOT NULL REFERENCES carpinterias(id),
    nombre          text NOT NULL,
    cuit            text,
    telefono        text,
    email           text,
    notas           text
);

-- ---------------------------------------------------------------------------
-- Productos y stock
-- ---------------------------------------------------------------------------

CREATE TYPE tipo_producto AS ENUM ('placa', 'canto', 'herraje', 'otro');

CREATE TABLE productos (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id  bigint NOT NULL REFERENCES carpinterias(id),
    tipo            tipo_producto NOT NULL,
    sku             text,
    nombre          text NOT NULL,            -- ej: 'Melamina Blanca 18mm'
    marca           text,
    color           text,
    unidad          text NOT NULL,            -- 'placa', 'metro', 'unidad'
    costo           numeric(14,2) NOT NULL DEFAULT 0,
    precio_venta    numeric(14,2) NOT NULL DEFAULT 0,
    stock_minimo    numeric(14,3) NOT NULL DEFAULT 0,
    -- Solo placas
    largo_mm        integer,                  -- en el sentido de la veta
    ancho_mm        integer,
    espesor_mm      numeric(5,1),
    tiene_veta      boolean NOT NULL DEFAULT false,
    refilado_mm     integer NOT NULL DEFAULT 0,  -- se descuenta de cada borde
    -- Solo cantos
    canto_ancho_mm   integer,
    canto_espesor_mm numeric(4,1),
    activo          boolean NOT NULL DEFAULT true,
    UNIQUE (carpinteria_id, sku),
    CHECK (tipo <> 'placa' OR (largo_mm > 0 AND ancho_mm > 0 AND espesor_mm > 0))
);

-- Saldo actual por depósito. Se mantiene en la misma transacción que el movimiento.
CREATE TABLE stock (
    carpinteria_id  bigint NOT NULL REFERENCES carpinterias(id),
    producto_id     bigint NOT NULL REFERENCES productos(id),
    deposito_id     bigint NOT NULL REFERENCES depositos(id),
    cantidad        numeric(14,3) NOT NULL DEFAULT 0,
    PRIMARY KEY (producto_id, deposito_id)
);

CREATE TYPE tipo_movimiento AS ENUM (
    'compra', 'venta', 'ajuste', 'traspaso_entrada', 'traspaso_salida', 'devolucion'
);

-- Libro de movimientos: explica cada cambio de stock.
CREATE TABLE stock_movimientos (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id  bigint NOT NULL REFERENCES carpinterias(id),
    producto_id     bigint NOT NULL REFERENCES productos(id),
    deposito_id     bigint NOT NULL REFERENCES depositos(id),
    tipo            tipo_movimiento NOT NULL,
    cantidad        numeric(14,3) NOT NULL,    -- positiva entra, negativa sale
    costo_unitario  numeric(14,2),
    pedido_id       bigint,
    compra_id       bigint,
    usuario_id      bigint REFERENCES usuarios(id),
    nota            text,
    creado_en       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON stock_movimientos (carpinteria_id, producto_id, creado_en);

-- Productos por debajo del stock mínimo, sumando todos los depósitos.
CREATE VIEW v_stock_bajo WITH (security_invoker = true) AS
SELECT p.carpinteria_id, p.id AS producto_id, p.nombre, p.stock_minimo,
       COALESCE(sum(s.cantidad), 0) AS cantidad_total
FROM productos p
LEFT JOIN stock s ON s.producto_id = p.id
WHERE p.activo
GROUP BY p.id
HAVING COALESCE(sum(s.cantidad), 0) < p.stock_minimo;

-- ---------------------------------------------------------------------------
-- Compras
-- ---------------------------------------------------------------------------

CREATE TABLE compras (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id  bigint NOT NULL REFERENCES carpinterias(id),
    proveedor_id    bigint NOT NULL REFERENCES proveedores(id),
    deposito_id     bigint NOT NULL REFERENCES depositos(id),
    numero_factura  text,
    fecha           date NOT NULL DEFAULT current_date,
    total           numeric(14,2) NOT NULL DEFAULT 0,
    usuario_id      bigint REFERENCES usuarios(id),
    creado_en       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE compra_items (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id  bigint NOT NULL REFERENCES carpinterias(id),
    compra_id       bigint NOT NULL REFERENCES compras(id) ON DELETE CASCADE,
    producto_id     bigint NOT NULL REFERENCES productos(id),
    cantidad        numeric(14,3) NOT NULL CHECK (cantidad > 0),
    costo_unitario  numeric(14,2) NOT NULL
);

-- ---------------------------------------------------------------------------
-- Muebles modelo (plantillas paramétricas)
-- ---------------------------------------------------------------------------
-- Las fórmulas usan las variables ancho, alto, profundidad y espesor (de la placa),
-- por ejemplo: 'alto', 'profundidad', 'ancho - 2*espesor'.

CREATE TABLE muebles_modelo (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id      bigint NOT NULL REFERENCES carpinterias(id),
    nombre              text NOT NULL,          -- ej: 'Bajo mesada 2 puertas'
    descripcion         text,
    ancho_mm            integer NOT NULL,       -- medidas por defecto
    alto_mm             integer NOT NULL,
    profundidad_mm      integer NOT NULL,
    placa_defecto_id    bigint REFERENCES productos(id),
    canto_defecto_id    bigint REFERENCES productos(id),
    precio_base         numeric(14,2),
    activo              boolean NOT NULL DEFAULT true
);

CREATE TABLE mueble_modelo_piezas (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id      bigint NOT NULL REFERENCES carpinterias(id),
    mueble_modelo_id    bigint NOT NULL REFERENCES muebles_modelo(id) ON DELETE CASCADE,
    nombre              text NOT NULL,          -- ej: 'Lateral'
    formula_largo       text NOT NULL,          -- en el sentido de la veta
    formula_ancho       text NOT NULL,
    cantidad            integer NOT NULL DEFAULT 1,
    placa_id            bigint REFERENCES productos(id),  -- NULL = placa del mueble
    respeta_veta        boolean NOT NULL DEFAULT true,
    -- Lados con canto: L1/L2 son los lados largos, A1/A2 los anchos.
    canto_l1            boolean NOT NULL DEFAULT false,
    canto_l2            boolean NOT NULL DEFAULT false,
    canto_a1            boolean NOT NULL DEFAULT false,
    canto_a2            boolean NOT NULL DEFAULT false,
    canto_id            bigint REFERENCES productos(id)   -- NULL = canto del mueble
);

CREATE TABLE mueble_modelo_herrajes (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id      bigint NOT NULL REFERENCES carpinterias(id),
    mueble_modelo_id    bigint NOT NULL REFERENCES muebles_modelo(id) ON DELETE CASCADE,
    producto_id         bigint NOT NULL REFERENCES productos(id),
    formula_cantidad    text NOT NULL DEFAULT '1'   -- ej: '4' bisagras
);

-- ---------------------------------------------------------------------------
-- Pedidos
-- ---------------------------------------------------------------------------

CREATE TYPE estado_pedido AS ENUM (
    'presupuesto', 'confirmado', 'en_corte', 'en_armado', 'listo', 'entregado', 'cancelado'
);

CREATE TABLE pedidos (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id  bigint NOT NULL REFERENCES carpinterias(id),
    numero          integer NOT NULL,            -- correlativo por carpintería
    sucursal_id     bigint NOT NULL REFERENCES sucursales(id),
    deposito_id     bigint NOT NULL REFERENCES depositos(id),  -- de dónde sale el stock
    cliente_id      bigint NOT NULL REFERENCES clientes(id),
    vendedor_id     bigint REFERENCES usuarios(id),
    estado          estado_pedido NOT NULL DEFAULT 'presupuesto',
    total           numeric(14,2) NOT NULL DEFAULT 0,
    costo_total     numeric(14,2) NOT NULL DEFAULT 0,   -- para calcular margen
    token_publico   text NOT NULL UNIQUE,                -- link del cliente al presupuesto
    fecha_entrega   date,
    notas           text,
    confirmado_en   timestamptz,
    entregado_en    timestamptz,
    creado_en       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (carpinteria_id, numero)
);
CREATE INDEX ON pedidos (carpinteria_id, estado);
CREATE INDEX ON pedidos (carpinteria_id, confirmado_en);

CREATE TABLE pedido_estados (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id  bigint NOT NULL REFERENCES carpinterias(id),
    pedido_id       bigint NOT NULL REFERENCES pedidos(id) ON DELETE CASCADE,
    estado          estado_pedido NOT NULL,
    usuario_id      bigint REFERENCES usuarios(id),
    creado_en       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE pedido_muebles (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id      bigint NOT NULL REFERENCES carpinterias(id),
    pedido_id           bigint NOT NULL REFERENCES pedidos(id) ON DELETE CASCADE,
    mueble_modelo_id    bigint REFERENCES muebles_modelo(id),  -- NULL = mueble a medida
    nombre              text NOT NULL,
    ancho_mm            integer,
    alto_mm             integer,
    profundidad_mm      integer,
    cantidad            integer NOT NULL DEFAULT 1,
    precio_unitario     numeric(14,2) NOT NULL DEFAULT 0
);

-- Despiece concreto de cada mueble del pedido (calculado del modelo o cargado a mano).
CREATE TABLE piezas (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id      bigint NOT NULL REFERENCES carpinterias(id),
    pedido_mueble_id    bigint NOT NULL REFERENCES pedido_muebles(id) ON DELETE CASCADE,
    nombre              text NOT NULL,
    placa_id            bigint NOT NULL REFERENCES productos(id),
    largo_mm            integer NOT NULL CHECK (largo_mm > 0),
    ancho_mm            integer NOT NULL CHECK (ancho_mm > 0),
    cantidad            integer NOT NULL DEFAULT 1 CHECK (cantidad > 0),
    respeta_veta        boolean NOT NULL DEFAULT true,
    canto_l1            boolean NOT NULL DEFAULT false,
    canto_l2            boolean NOT NULL DEFAULT false,
    canto_a1            boolean NOT NULL DEFAULT false,
    canto_a2            boolean NOT NULL DEFAULT false,
    canto_id            bigint REFERENCES productos(id)
);

-- Herrajes y productos sueltos del pedido (incluye los herrajes de los muebles).
-- Es la base de la estadística de productos más vendidos.
CREATE TABLE pedido_items (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id      bigint NOT NULL REFERENCES carpinterias(id),
    pedido_id           bigint NOT NULL REFERENCES pedidos(id) ON DELETE CASCADE,
    pedido_mueble_id    bigint REFERENCES pedido_muebles(id) ON DELETE CASCADE,
    producto_id         bigint NOT NULL REFERENCES productos(id),
    cantidad            numeric(14,3) NOT NULL CHECK (cantidad > 0),
    precio_unitario     numeric(14,2) NOT NULL DEFAULT 0,
    costo_unitario      numeric(14,2) NOT NULL DEFAULT 0
);
CREATE INDEX ON pedido_items (carpinteria_id, producto_id);

CREATE TYPE tipo_pago AS ENUM ('sena', 'pago', 'devolucion');

CREATE TABLE pagos (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id  bigint NOT NULL REFERENCES carpinterias(id),
    pedido_id       bigint NOT NULL REFERENCES pedidos(id),
    tipo            tipo_pago NOT NULL,
    monto           numeric(14,2) NOT NULL CHECK (monto > 0),
    medio           text NOT NULL,       -- efectivo, transferencia, tarjeta...
    usuario_id      bigint REFERENCES usuarios(id),
    nota            text,
    creado_en       timestamptz NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- Planes de corte y retazos
-- ---------------------------------------------------------------------------

CREATE TABLE planes_corte (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id  bigint NOT NULL REFERENCES carpinterias(id),
    pedido_id       bigint REFERENCES pedidos(id) ON DELETE CASCADE,
    placa_id        bigint NOT NULL REFERENCES productos(id),
    kerf_mm         integer NOT NULL DEFAULT 3,
    refilado_mm     integer NOT NULL DEFAULT 0,
    aprovechamiento numeric(5,2),        -- % de área usada
    creado_por      bigint REFERENCES usuarios(id),
    creado_en       timestamptz NOT NULL DEFAULT now()
);

CREATE TYPE estado_retazo AS ENUM ('disponible', 'reservado', 'usado', 'descartado');

CREATE TABLE retazos (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id      bigint NOT NULL REFERENCES carpinterias(id),
    placa_id            bigint NOT NULL REFERENCES productos(id),
    deposito_id         bigint NOT NULL REFERENCES depositos(id),
    largo_mm            integer NOT NULL CHECK (largo_mm > 0),
    ancho_mm            integer NOT NULL CHECK (ancho_mm > 0),
    estado              estado_retazo NOT NULL DEFAULT 'disponible',
    plan_origen_id      bigint REFERENCES planes_corte(id),
    creado_en           timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON retazos (carpinteria_id, placa_id, estado);

-- Cada placa (entera o retazo) que usa el plan.
CREATE TABLE plan_placas (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id  bigint NOT NULL REFERENCES carpinterias(id),
    plan_id         bigint NOT NULL REFERENCES planes_corte(id) ON DELETE CASCADE,
    indice          integer NOT NULL,
    retazo_id       bigint REFERENCES retazos(id),   -- NULL = placa entera
    largo_mm        integer NOT NULL,
    ancho_mm        integer NOT NULL,
    aprovechamiento numeric(5,2),
    UNIQUE (plan_id, indice)
);

-- Posición de cada pieza dentro de la placa (origen abajo a la izquierda).
CREATE TABLE plan_piezas (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id  bigint NOT NULL REFERENCES carpinterias(id),
    plan_placa_id   bigint NOT NULL REFERENCES plan_placas(id) ON DELETE CASCADE,
    pieza_id        bigint REFERENCES piezas(id),
    etiqueta        text NOT NULL,
    x_mm            integer NOT NULL,
    y_mm            integer NOT NULL,
    largo_mm        integer NOT NULL,   -- tal como quedó colocada
    ancho_mm        integer NOT NULL,
    rotada          boolean NOT NULL DEFAULT false
);

-- ---------------------------------------------------------------------------
-- Row Level Security
-- ---------------------------------------------------------------------------

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY[
    'sucursales','depositos','usuarios','roles','rol_permisos','usuario_roles',
    'clientes','proveedores','productos','stock','stock_movimientos',
    'compras','compra_items','muebles_modelo','mueble_modelo_piezas',
    'mueble_modelo_herrajes','pedidos','pedido_estados','pedido_muebles','piezas',
    'pedido_items','pagos','planes_corte','retazos','plan_placas','plan_piezas'
  ] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format(
      'CREATE POLICY aislamiento ON %I USING (carpinteria_id = app_carpinteria_id()) '
      'WITH CHECK (carpinteria_id = app_carpinteria_id())', t);
  END LOOP;
END $$;
-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- Permisos base
-- ---------------------------------------------------------------------------

INSERT INTO permisos (codigo, descripcion) VALUES
  ('usuarios.ver',        'Ver usuarios'),
  ('usuarios.editar',     'Crear y editar usuarios y roles'),
  ('sucursales.editar',   'Crear y editar sucursales y depósitos'),
  ('productos.ver',       'Ver productos'),
  ('productos.editar',    'Crear y editar productos y precios'),
  ('stock.ver',           'Ver stock'),
  ('stock.editar',        'Ajustar stock y hacer traspasos'),
  ('compras.ver',         'Ver compras y proveedores'),
  ('compras.editar',      'Registrar compras y proveedores'),
  ('clientes.ver',        'Ver clientes'),
  ('clientes.editar',     'Crear y editar clientes'),
  ('pedidos.ver',         'Ver pedidos'),
  ('pedidos.editar',      'Crear y editar presupuestos y pedidos'),
  ('pedidos.confirmar',   'Confirmar pedidos (descuenta stock)'),
  ('pedidos.estado',      'Cambiar el estado de producción'),
  ('pagos.registrar',     'Registrar señas y pagos'),
  ('muebles.editar',      'Crear y editar muebles modelo'),
  ('cortes.ver',          'Ver planes de corte'),
  ('cortes.generar',      'Generar planes de corte'),
  ('estadisticas.ver',    'Ver estadísticas de ventas');

-- +goose Down
DROP VIEW IF EXISTS v_stock_bajo;
DROP TABLE IF EXISTS plan_piezas, plan_placas, retazos, planes_corte, pagos,
  pedido_items, piezas, pedido_muebles, pedido_estados, pedidos,
  mueble_modelo_herrajes, mueble_modelo_piezas, muebles_modelo,
  compra_items, compras, stock_movimientos, stock, productos,
  proveedores, clientes, usuario_roles, rol_permisos, roles, usuarios,
  permisos, depositos, sucursales, carpinterias CASCADE;
DROP TYPE IF EXISTS estado_retazo, tipo_pago, estado_pedido, tipo_movimiento, tipo_producto;
DROP FUNCTION IF EXISTS app_carpinteria_id();
