# Diseño de Ñandubay

## Alcance acordado

- Varias carpinterías, cada una con sus datos separados. Cada carpintería tiene
  varias sucursales y cada sucursal uno o más depósitos.
- Usuarios con roles armables a medida. Roles base: admin, vendedor, operario, depósito.
  Un usuario puede tener un rol distinto en cada sucursal.
- Los clientes finales solo ven sus presupuestos, con un link seguro (sin usuario).
- Stock por depósito, con proveedores, compras y costos, alerta de stock mínimo
  y reutilización de retazos. El stock se descuenta al confirmar el pedido.
- Pedidos con varios muebles. Muebles modelo reutilizables con despiece paramétrico.
- Estados del pedido: presupuesto, confirmado, en corte, en armado, listo, entregado (y cancelado).
- Señas y pagos parciales como registro interno (sin facturación electrónica).
- Estadísticas: productos más vendidos, ventas por mes y por vendedor, margen, consumo de placas.
- Optimizador de cortes para escuadradora (guillotina), sierra de 3 mm, veta en
  algunas placas, cantos por lado de pieza y cálculo de metros de canto.
  Carga manual desde la web, visualización e impresión.
- Uso desde computadora y celular. Despliegue en un VPS.

## Arquitectura

```
Navegador (React SPA)
      │ HTTPS
   Caddy ── sirve el frontend compilado
      │ /api/*
   API Go (chi, pgx) ── optimizador de cortes (internal/cortes)
      │
   PostgreSQL (Row Level Security por carpintería)
```

- **Aislamiento entre carpinterías**: todas las tablas de datos tienen
  `carpinteria_id` y una política RLS. La API abre cada transacción con
  `db.EnCarpinteria(...)`, que fija `app.carpinteria_id`; Postgres filtra el resto.
  Si un handler se olvida un `WHERE`, igual no puede ver datos de otra carpintería.
  La API se conecta con un usuario sin superusuario (los superusuarios se saltean RLS).
- **Medidas** en milímetros enteros. En las placas, `largo` es el sentido de la veta.
- **Stock**: la tabla `stock` guarda el saldo por depósito y `stock_movimientos`
  explica cada cambio. Ambas se actualizan en la misma transacción.

## Modelo de datos

Ver `backend/migrations/00001_esquema_inicial.sql`. Resumen:

| Área | Tablas |
|---|---|
| Organización | `carpinterias`, `sucursales`, `depositos` |
| Acceso | `usuarios`, `permisos` (catálogo fijo), `roles`, `rol_permisos`, `usuario_roles` |
| Terceros | `clientes`, `proveedores` |
| Stock | `productos` (placa, canto, herraje, otro), `stock`, `stock_movimientos`, vista `v_stock_bajo` |
| Compras | `compras`, `compra_items` |
| Muebles modelo | `muebles_modelo`, `mueble_modelo_piezas` (fórmulas), `mueble_modelo_herrajes` |
| Pedidos | `pedidos`, `pedido_estados`, `pedido_muebles`, `piezas`, `pedido_items`, `pagos` |
| Cortes | `planes_corte`, `plan_placas`, `plan_piezas`, `retazos` |

### Muebles paramétricos

Cada pieza de un mueble modelo tiene fórmulas para largo y ancho con las variables
`ancho`, `alto`, `profundidad` y `espesor`. Ejemplo para un bajo mesada:

| Pieza | Largo | Ancho | Cant. |
|---|---|---|---|
| Lateral | `alto` | `profundidad` | 2 |
| Piso | `ancho - 2*espesor` | `profundidad` | 1 |
| Puerta | `alto - 4` | `ancho/2 - 4` | 2 |

Al agregar el mueble a un pedido con sus medidas, se calculan las fórmulas y se
guardan las piezas concretas en `piezas`. Así se pueden retocar a mano.

## Optimizador de cortes

`backend/internal/cortes`. Algoritmo guillotina con "mejor ajuste por área":

1. Expande las piezas por cantidad y las ordena de mayor a menor.
2. Para cada pieza busca, entre las placas ya abiertas, el hueco libre donde sobra
   menos. Prueba la pieza rotada solo si la placa no tiene veta o la pieza no la respeta.
3. Si no entra en ninguna, abre primero el retazo disponible más chico donde entre,
   y si no hay, una placa entera (descontando el refilado de cada borde).
4. Al colocar una pieza, el hueco se parte en dos con un corte de lado a lado,
   dejando el espesor de la sierra entre piezas.
5. Los huecos que quedan por encima de una medida mínima se proponen como retazos nuevos.

`POST /api/v1/cortes/optimizar` recibe parámetros y piezas y devuelve las placas
con la posición de cada pieza, los sobrantes, el aprovechamiento y los metros de canto.

Mejoras previstas: probar varios órdenes y quedarse con el mejor, secuencia de
cortes para el operario, y guardar el plan en `planes_corte` desde un pedido.

## Próximos pasos

1. Login y sesiones. Pendiente decidir cómo se identifica la carpintería al
   entrar: email único global (más simple) o subdominio por carpintería.
2. ABM de productos, depósitos y stock con movimientos.
3. Pedidos y muebles modelo con cálculo de fórmulas.
4. Guardar planes de corte y generar retazos.
5. Estadísticas.
6. Consultas con sqlc y tipos del frontend generados desde OpenAPI.
