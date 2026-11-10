-- +goose Up
-- Las ventas de mostrador (productos del stock) y los presupuestos de muebles
-- van en secciones separadas.
ALTER TABLE pedidos ADD COLUMN tipo text NOT NULL DEFAULT 'mostrador'
    CHECK (tipo IN ('mostrador', 'muebles'));

-- Las ventas que ya tienen muebles cargados pasan a ser de muebles. Se saca
-- FORCE un momento para poder tocar las de todas las carpinterías.
ALTER TABLE pedidos NO FORCE ROW LEVEL SECURITY;
ALTER TABLE pedido_muebles NO FORCE ROW LEVEL SECURITY;
UPDATE pedidos p SET tipo = 'muebles'
WHERE EXISTS (SELECT 1 FROM pedido_muebles m WHERE m.pedido_id = p.id);
ALTER TABLE pedido_muebles FORCE ROW LEVEL SECURITY;
ALTER TABLE pedidos FORCE ROW LEVEL SECURITY;

CREATE INDEX ON pedidos (carpinteria_id, tipo, numero);
CREATE INDEX ON pedido_items (pedido_id) WHERE pedido_mueble_id IS NULL;

-- +goose Down
DROP INDEX IF EXISTS pedido_items_pedido_id_idx;
ALTER TABLE pedidos DROP COLUMN tipo;
