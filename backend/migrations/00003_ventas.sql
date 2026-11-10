-- +goose Up
-- Ventas: presupuestos, confirmación con descuento de stock, pagos, retiros
-- en partes y boleta.

-- El depósito de salida se elige al confirmar, no al presupuestar. Puede ser de
-- otra sucursal: la mercadería se retira en la sucursal de ese depósito.
ALTER TABLE pedidos ALTER COLUMN deposito_id DROP NOT NULL;

ALTER TABLE clientes ADD COLUMN documento text;   -- DNI o CUIT, para la boleta
CREATE INDEX ON clientes (carpinteria_id, lower(nombre));

-- Cuánto se llevó ya el cliente de cada línea.
ALTER TABLE pedido_items ADD COLUMN retirado numeric(14,3) NOT NULL DEFAULT 0
    CHECK (retirado >= 0 AND retirado <= cantidad);
ALTER TABLE pedido_items ADD COLUMN descripcion text;
ALTER TABLE pedido_items ADD COLUMN orden integer NOT NULL DEFAULT 0;
ALTER TABLE pedido_muebles ADD COLUMN retirado integer NOT NULL DEFAULT 0
    CHECK (retirado >= 0 AND retirado <= cantidad);
ALTER TABLE pedido_muebles ADD COLUMN orden integer NOT NULL DEFAULT 0;
ALTER TABLE pedido_muebles ADD CONSTRAINT pedido_muebles_cantidad_positiva CHECK (cantidad > 0);

-- Cada vez que el cliente pasa a buscar mercadería con la boleta.
CREATE TABLE retiros (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id  bigint NOT NULL REFERENCES carpinterias(id),
    pedido_id       bigint NOT NULL REFERENCES pedidos(id),
    sucursal_id     bigint NOT NULL REFERENCES sucursales(id),
    usuario_id      bigint REFERENCES usuarios(id),
    nota            text,
    creado_en       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON retiros (pedido_id);

CREATE TABLE retiro_items (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id      bigint NOT NULL REFERENCES carpinterias(id),
    retiro_id           bigint NOT NULL REFERENCES retiros(id) ON DELETE CASCADE,
    pedido_item_id      bigint REFERENCES pedido_items(id),
    pedido_mueble_id    bigint REFERENCES pedido_muebles(id),
    cantidad            numeric(14,3) NOT NULL CHECK (cantidad > 0),
    CHECK ((pedido_item_id IS NULL) <> (pedido_mueble_id IS NULL))
);

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['retiros', 'retiro_items'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format(
      'CREATE POLICY aislamiento ON %I USING (carpinteria_id = app_carpinteria_id()) '
      'WITH CHECK (carpinteria_id = app_carpinteria_id())', t);
  END LOOP;
END $$;
-- +goose StatementEnd

INSERT INTO permisos (codigo, descripcion) VALUES
  ('retiros.registrar', 'Entregar mercadería contra boleta');

-- Las carpinterías que ya existen reciben el permiso nuevo en los roles que
-- lo traen de fábrica. La migración corre como dueña de las tablas, así que
-- se saca FORCE un momento para poder leer y escribir los datos de todas.
ALTER TABLE roles NO FORCE ROW LEVEL SECURITY;
ALTER TABLE rol_permisos NO FORCE ROW LEVEL SECURITY;
INSERT INTO rol_permisos (carpinteria_id, rol_id, permiso)
SELECT r.carpinteria_id, r.id, 'retiros.registrar'
FROM roles r WHERE r.nombre IN ('Admin', 'Vendedor', 'Depósito')
ON CONFLICT DO NOTHING;
INSERT INTO rol_permisos (carpinteria_id, rol_id, permiso)
SELECT r.carpinteria_id, r.id, 'pedidos.ver'
FROM roles r WHERE r.nombre = 'Depósito'
ON CONFLICT DO NOTHING;
ALTER TABLE rol_permisos FORCE ROW LEVEL SECURITY;
ALTER TABLE roles FORCE ROW LEVEL SECURITY;

-- +goose Down
ALTER TABLE rol_permisos NO FORCE ROW LEVEL SECURITY;
DELETE FROM rol_permisos WHERE permiso = 'retiros.registrar';
ALTER TABLE rol_permisos FORCE ROW LEVEL SECURITY;
DELETE FROM permisos WHERE codigo = 'retiros.registrar';
DROP TABLE retiro_items, retiros;
ALTER TABLE pedido_muebles DROP CONSTRAINT pedido_muebles_cantidad_positiva;
ALTER TABLE pedido_muebles DROP COLUMN orden, DROP COLUMN retirado;
ALTER TABLE pedido_items DROP COLUMN orden, DROP COLUMN descripcion, DROP COLUMN retirado;
ALTER TABLE clientes DROP COLUMN documento;
