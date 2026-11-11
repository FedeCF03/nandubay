-- +goose Up
-- Cortes optimizados: un plan de corte guardado (desde el optimizador o al
-- confirmar un presupuesto de muebles) queda pendiente hasta que en el taller
-- se confirma que se cortó. Recién ahí se descuentan las placas enteras, se
-- marcan los retazos usados y los sobrantes se guardan como retazos nuevos.
ALTER TABLE planes_corte
    ADD COLUMN nombre      text NOT NULL DEFAULT '',
    ADD COLUMN deposito_id bigint REFERENCES depositos(id),
    ADD COLUMN estado      text NOT NULL DEFAULT 'pendiente'
        CHECK (estado IN ('pendiente', 'cortado', 'anulado')),
    ADD COLUMN parametros  jsonb NOT NULL DEFAULT '{}',
    ADD COLUMN piezas      jsonb NOT NULL DEFAULT '[]',
    ADD COLUMN resultado   jsonb NOT NULL DEFAULT '{}',
    ADD COLUMN cortado_en  timestamptz,
    ADD COLUMN cortado_por bigint REFERENCES usuarios(id);
CREATE INDEX ON planes_corte (carpinteria_id, estado, creado_en DESC);
CREATE INDEX ON planes_corte (pedido_id) WHERE pedido_id IS NOT NULL;

ALTER TABLE retazos ADD COLUMN plan_uso_id bigint REFERENCES planes_corte(id);

-- El descuento de placas al cortar es un movimiento propio (no "venta"), así
-- cancelar la venta no devuelve placas que ya se cortaron.
ALTER TYPE tipo_movimiento ADD VALUE IF NOT EXISTS 'corte';

INSERT INTO permisos (codigo, descripcion) VALUES
  ('cortes.confirmar', 'Confirmar cortes hechos (descuenta placas y carga retazos)');

ALTER TABLE roles NO FORCE ROW LEVEL SECURITY;
ALTER TABLE rol_permisos NO FORCE ROW LEVEL SECURITY;
INSERT INTO rol_permisos (carpinteria_id, rol_id, permiso)
SELECT r.carpinteria_id, r.id, 'cortes.confirmar'
FROM roles r WHERE r.nombre IN ('Admin', 'Operario', 'Depósito')
ON CONFLICT DO NOTHING;
ALTER TABLE rol_permisos FORCE ROW LEVEL SECURITY;
ALTER TABLE roles FORCE ROW LEVEL SECURITY;

-- +goose Down
ALTER TABLE rol_permisos NO FORCE ROW LEVEL SECURITY;
DELETE FROM rol_permisos WHERE permiso = 'cortes.confirmar';
ALTER TABLE rol_permisos FORCE ROW LEVEL SECURITY;
DELETE FROM permisos WHERE codigo = 'cortes.confirmar';
ALTER TABLE retazos DROP COLUMN plan_uso_id;
ALTER TABLE planes_corte DROP COLUMN nombre, DROP COLUMN deposito_id, DROP COLUMN estado,
    DROP COLUMN parametros, DROP COLUMN piezas, DROP COLUMN resultado,
    DROP COLUMN cortado_en, DROP COLUMN cortado_por;
