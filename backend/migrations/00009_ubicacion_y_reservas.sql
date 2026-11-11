-- +goose Up
-- Ubicación del stock y descuento al retirar.
--
-- 1. Cada producto tiene una ubicación dentro de cada depósito (estante,
--    pasillo, rack...). Va en la fila de saldo de ese producto en ese depósito.
-- 2. En las ventas de mostrador el stock ya no se descuenta al confirmar: se
--    descuenta cuando el cliente retira (también si retira por partes).
--    Mientras tanto, si la venta tiene algo pagado (seña o pago), lo que falta
--    retirar queda reservado hasta reservado_hasta (2 meses desde el pago) y
--    no se le puede vender a otro cliente.
ALTER TABLE stock ADD COLUMN ubicacion text
    CHECK (ubicacion IS NULL OR char_length(ubicacion) <= 60);

ALTER TABLE pedidos ADD COLUMN reservado_hasta timestamptz;

-- Lo reservado por las ventas de mostrador con la reserva vigente: lo que
-- falta retirar de cada producto, por venta y depósito.
CREATE VIEW v_stock_reservado WITH (security_invoker = true) AS
SELECT p.carpinteria_id, p.id AS pedido_id, p.deposito_id, i.producto_id,
       sum(i.cantidad - i.retirado) AS cantidad
FROM pedidos p
JOIN pedido_items i ON i.pedido_id = p.id
WHERE p.tipo = 'mostrador'
  AND p.estado IN ('confirmado', 'en_corte', 'en_armado', 'listo')
  AND p.deposito_id IS NOT NULL
  AND p.reservado_hasta > now()
  AND i.pedido_mueble_id IS NULL
  AND i.producto_id IS NOT NULL
  AND i.cantidad > i.retirado
GROUP BY p.carpinteria_id, p.id, p.deposito_id, i.producto_id;

CREATE INDEX ON pedidos (carpinteria_id, reservado_hasta) WHERE reservado_hasta IS NOT NULL;

-- Las ventas de mostrador ya confirmadas descontaron todo al confirmar. Con la
-- regla nueva solo tiene que estar descontado lo que ya se retiró: lo que
-- falta retirar vuelve al depósito (queda registrado como devolución) y, si
-- la venta tiene algo pagado, queda reservado 2 meses desde hoy.
-- Se saca FORCE un momento para tocar los datos de todas las carpinterías.
ALTER TABLE pedidos NO FORCE ROW LEVEL SECURITY;
ALTER TABLE pedido_items NO FORCE ROW LEVEL SECURITY;
ALTER TABLE pagos NO FORCE ROW LEVEL SECURITY;
ALTER TABLE stock NO FORCE ROW LEVEL SECURITY;
ALTER TABLE stock_movimientos NO FORCE ROW LEVEL SECURITY;

UPDATE pedidos p SET reservado_hasta = now() + interval '2 months'
WHERE p.tipo = 'mostrador'
  AND p.estado IN ('confirmado', 'en_corte', 'en_armado', 'listo')
  AND p.deposito_id IS NOT NULL
  AND (SELECT coalesce(sum(CASE WHEN g.tipo = 'devolucion' THEN -g.monto ELSE g.monto END), 0)
       FROM pagos g WHERE g.pedido_id = p.id) > 0;

CREATE TEMP TABLE pendientes_09 ON COMMIT DROP AS
SELECT p.carpinteria_id, p.id AS pedido_id, p.numero, p.deposito_id, i.producto_id,
       sum(i.cantidad - i.retirado) AS cantidad
FROM pedidos p
JOIN pedido_items i ON i.pedido_id = p.id
WHERE p.tipo = 'mostrador'
  AND p.estado IN ('confirmado', 'en_corte', 'en_armado', 'listo')
  AND p.deposito_id IS NOT NULL
  AND i.pedido_mueble_id IS NULL
  AND i.producto_id IS NOT NULL
GROUP BY p.carpinteria_id, p.id, p.numero, p.deposito_id, i.producto_id
HAVING sum(i.cantidad - i.retirado) > 0;

INSERT INTO stock_movimientos (carpinteria_id, producto_id, deposito_id, tipo, cantidad, pedido_id, nota)
SELECT carpinteria_id, producto_id, deposito_id, 'devolucion', cantidad, pedido_id,
       'Venta N° ' || numero || ': ahora el stock se descuenta al retirar'
FROM pendientes_09;

INSERT INTO stock (carpinteria_id, producto_id, deposito_id, cantidad)
SELECT carpinteria_id, producto_id, deposito_id, sum(cantidad)
FROM pendientes_09
GROUP BY carpinteria_id, producto_id, deposito_id
ON CONFLICT (producto_id, deposito_id) DO UPDATE SET cantidad = stock.cantidad + EXCLUDED.cantidad;

ALTER TABLE stock_movimientos FORCE ROW LEVEL SECURITY;
ALTER TABLE stock FORCE ROW LEVEL SECURITY;
ALTER TABLE pagos FORCE ROW LEVEL SECURITY;
ALTER TABLE pedido_items FORCE ROW LEVEL SECURITY;
ALTER TABLE pedidos FORCE ROW LEVEL SECURITY;

-- +goose Down
-- Lo devuelto al stock por esta migración no se vuelve a descontar.
DROP VIEW v_stock_reservado;
DROP INDEX IF EXISTS pedidos_carpinteria_id_reservado_hasta_idx;
ALTER TABLE pedidos DROP COLUMN reservado_hasta;
ALTER TABLE stock DROP COLUMN ubicacion;
