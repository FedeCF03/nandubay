-- +goose Up
-- Retazos: de qué venta salió cada uno (el sobrante de sus cortes) y en cuál
-- se usó, para poder revertirlo si la venta se cancela.
ALTER TABLE retazos
    ADD COLUMN pedido_origen_id bigint REFERENCES pedidos(id),
    ADD COLUMN pedido_uso_id    bigint REFERENCES pedidos(id),
    ADD COLUMN nota             text,
    ADD COLUMN usado_en         timestamptz;
CREATE INDEX ON retazos (pedido_origen_id) WHERE pedido_origen_id IS NOT NULL;
CREATE INDEX ON retazos (pedido_uso_id) WHERE pedido_uso_id IS NOT NULL;

-- Plan de corte con el que se confirmó un presupuesto de muebles (placas,
-- retazos usados, canto e insumos). Después de confirmar se muestra este y no
-- un recálculo, porque los retazos disponibles ya cambiaron.
ALTER TABLE pedidos ADD COLUMN materiales jsonb;

-- +goose Down
ALTER TABLE pedidos DROP COLUMN materiales;
ALTER TABLE retazos DROP COLUMN pedido_origen_id, DROP COLUMN pedido_uso_id, DROP COLUMN nota, DROP COLUMN usado_en;
