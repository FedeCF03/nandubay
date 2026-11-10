-- +goose Up
-- Cada mueble del presupuesto tiene una clave que se mantiene al editarlo
-- (las filas de pedido_muebles se vuelven a crear en cada edición), así sus
-- fotos no se pierden.
ALTER TABLE pedido_muebles ADD COLUMN clave uuid NOT NULL DEFAULT gen_random_uuid();

CREATE TABLE mueble_fotos (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id  bigint NOT NULL REFERENCES carpinterias(id),
    pedido_id       bigint NOT NULL REFERENCES pedidos(id) ON DELETE CASCADE,
    mueble_clave    uuid NOT NULL,
    tipo            text NOT NULL CHECK (tipo IN ('image/jpeg', 'image/png', 'image/webp')),
    datos           bytea NOT NULL,
    usuario_id      bigint REFERENCES usuarios(id),
    creado_en       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON mueble_fotos (pedido_id, mueble_clave);

ALTER TABLE mueble_fotos ENABLE ROW LEVEL SECURITY;
ALTER TABLE mueble_fotos FORCE ROW LEVEL SECURITY;
CREATE POLICY aislamiento ON mueble_fotos USING (carpinteria_id = app_carpinteria_id())
    WITH CHECK (carpinteria_id = app_carpinteria_id());

-- +goose Down
DROP TABLE mueble_fotos;
ALTER TABLE pedido_muebles DROP COLUMN clave;
