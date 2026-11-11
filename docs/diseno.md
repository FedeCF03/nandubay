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

Monolito modular con **clean architecture** en el backend:

```
Navegador (React SPA)
      │ HTTPS
   Caddy ── sirve el frontend compilado
      │ /api/*
   API Go ── PostgreSQL (Row Level Security por carpintería)
```

### Capas del backend

```
backend/
├── cmd/                       Arranque: arma las capas y las conecta
│   ├── api/                   Servidor HTTP
│   └── nandubay-admin/        Comandos de administración
├── internal/
│   ├── dominio/               Entidades y reglas de negocio puras
│   │   ├── acceso/            Usuarios, sucursales, permisos, roles base
│   │   ├── stock/             Productos, depósitos, ingresos, ajustes, traspasos
│   │   └── cortes/            Optimizador de cortes y metros de canto
│   ├── aplicacion/            Casos de uso y puertos (interfaces)
│   │   ├── puertos.go         Lo que la aplicación necesita del exterior
│   │   ├── acceso.go          Login, sesiones, alta de carpinterías
│   │   ├── stock.go           Productos y movimientos de stock
│   │   └── cortes.go          Optimizar cortes
│   ├── adaptadores/           Implementan los puertos o exponen los casos de uso
│   │   ├── api/               HTTP: rutas, sesión, permisos, JSON
│   │   └── postgres/          Repositorios, conexión, migraciones, RLS
│   └── plataforma/            Detalles técnicos
│       ├── config/            Variables de entorno
│       └── seguridad/         bcrypt y tokens de sesión
└── migrations/                SQL versionado (goose)
```

**Regla de dependencias**: cada capa solo conoce a las de adentro.

- `dominio` no importa nada del proyecto ni librerías externas.
- `aplicacion` importa solo `dominio`. Habla con la base, el hash de
  contraseñas, etc. a través de interfaces (`RepositorioAcceso`, `Hasher`, `Tokens`).
- `adaptadores` y `plataforma` implementan esas interfaces.
- `cmd` es el único lugar que conoce todo y arma las piezas.

`arquitectura_test.go` controla esta regla: si alguien importa pgx o el
adaptador HTTP desde el dominio o la aplicación, el test falla.

**Tests por capa**

- Dominio: tests unitarios puros (optimizador, resolución de permisos).
- Aplicación: tests con dobles en memoria, sin Postgres.
- Adaptadores: tests de integración contra Postgres real (`NANDUBAY_TEST_DB`).

**Cómo sumar un módulo nuevo** (por ejemplo stock):

1. `dominio/stock`: entidades (Producto, Movimiento) y reglas.
2. `aplicacion/stock.go`: casos de uso y el puerto `RepositorioStock` en `puertos.go`.
3. `adaptadores/postgres/repo_stock.go`: implementación con SQL.
4. `adaptadores/api/stock.go`: rutas HTTP con su permiso.
5. Conectarlo en `cmd/api/main.go`.

### Otras decisiones

- **Aislamiento entre carpinterías**: todas las tablas de datos tienen
  `carpinteria_id` y una política RLS. Los repositorios abren cada transacción con
  `postgres.EnCarpinteria(...)`, que fija `app.carpinteria_id`; Postgres filtra el resto.
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
| Stock | `productos` (placa, canto, herraje, otro), `categorias_producto`, `stock`, `stock_movimientos`, vista `v_stock_bajo` |
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

`backend/internal/dominio/cortes`. Algoritmo guillotina con "mejor ajuste por área":

1. Expande las piezas por cantidad y las ordena de mayor a menor.
2. Para cada pieza busca, entre las placas ya abiertas, el hueco libre donde sobra
   menos. Prueba la pieza rotada solo si la placa no tiene veta o la pieza no la respeta.
   Los retazos abiertos van primero, así se llenan antes de tocar una placa entera.
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

1. ~~Login y sesiones~~ (hecho, ver "Login y permisos").
2. ~~Productos y stock con movimientos~~ (hecho, ver "Stock").
3. ~~Ventas, pagos, boletas y retiros~~ (hecho, ver "Ventas y boletas"). Falta: muebles modelo con cálculo de fórmulas.
4. ~~Generar y reutilizar retazos~~ (hecho, ver "Retazos").
5. Estadísticas.
6. Tipos del frontend generados desde OpenAPI.

## Ventas y boletas

Hay dos secciones: **ventas de mostrador** (solo productos del stock: placas, cantos,
herrajes) y **presupuestos de muebles** (ver más abajo). Las dos usan la tabla
`pedidos` con la columna `tipo` (`mostrador` o `muebles`) y comparten boleta, pagos y
retiros. El mostrador tiene un resumen de **productos vendidos** por período y sucursal
(solo renglones sueltos, no los insumos de un mueble), con la ganancia sobre el costo.

- Una venta nace como **presupuesto** en la sucursal del vendedor. Tiene renglones de
  productos del stock y de muebles a medida. Mientras es presupuesto se edita libremente.
- **Confirmar** elige el depósito de salida (puede ser de otra sucursal) y descuenta el
  stock en la misma transacción, con movimientos tipo `venta` ligados al pedido. Si no
  alcanza el stock, no se confirma nada.
- **Pagos**: seña, pago y devolución. No se puede cobrar más que el saldo.
- **Retiros**: el cliente retira con la boleta en la sucursal del depósito de salida, de
  una vez o por partes. Para retirar la venta tiene que estar paga; los muebles se
  entregan cuando están en estado "lista". Cuando se retira todo, pasa sola a "entregada".
  Cada retiro guarda qué se llevó; nunca se puede retirar más de lo vendido.
- Estados de producción (confirmada, en corte, en armado, lista) se cambian a mano.
- **Cancelar**: solo sin retiros y sin plata cobrada (primero se registra la devolución).
  Si estaba confirmada, el stock vuelve al depósito.
- **Boleta A4**: dice PAGADA o SALDO PENDIENTE, dónde se retira, lo que queda por
  retirar y un QR a la venta. Lleva "X / Documento no válido como factura": no es una
  factura fiscal (no hay integración con AFIP).
- El dominio devuelve un `ventas.Cambio` y el repositorio lo aplica con la venta
  bloqueada (`SELECT … FOR UPDATE`), así dos operaciones a la vez no se pisan.
- Permisos: `pedidos.ver/editar/confirmar/estado`, `pagos.registrar` y el nuevo
  `retiros.registrar` (Admin, Vendedor y Depósito).

### Presupuestos de muebles

Un presupuesto de muebles (`tipo = 'muebles'`) tiene uno o más muebles, y cada mueble:

- sus **piezas** (tabla `piezas`): nombre opcional, placa del stock, largo × ancho,
  cantidad por mueble, si respeta la veta, los lados con canto (L1 L2 A1 A2) y qué canto.
  Cada pieza puede ser de una placa distinta (ej: 5 piezas blancas y 6 negras);
- sus **herrajes e insumos** (`pedido_items` con `pedido_mueble_id`): bisagras, patas,
  guías… por unidad de mueble. No se listan como renglones sueltos de la venta.

`muebles.Calcular` junta las piezas de **todos** los muebles por placa (multiplicadas por
la cantidad de cada mueble) y corre el optimizador una vez por placa, con la medida y el
refilado del producto y sierra de 3 mm. Suma los metros de canto por producto (30 mm de
merma por tramo) y los insumos, y da el costo total y el costo de cada mueble (las placas
se reparten según la superficie de sus piezas). Los materiales no se guardan: se
recalculan al ver el presupuesto, así siempre usan el costo actual.

El precio de cada mueble se escribe a mano; si se deja vacío se usa el sugerido, costo +
margen (el margen se recuerda en el navegador). API: `POST /presupuestos/calcular` (sin
guardar, para ir viendo mientras se carga), `POST /presupuestos`, `GET` y
`PUT /presupuestos/{id}`. Pagos, confirmación, boleta y retiros son los de cualquier venta.

**Fotos**: cada mueble puede tener hasta 12 fotos (tabla `mueble_fotos`, en la base,
con RLS). El navegador las achica a JPEG de 1600 px antes de subirlas y viajan en base64
dentro del JSON, así sigue valiendo la protección CSRF. El servidor detecta el tipo por
los bytes (solo JPG, PNG o WebP). Las fotos se atan a la `clave` (uuid) del mueble, que se
mantiene al editar el presupuesto; si se saca un mueble se borran sus fotos.
`POST /presupuestos/{id}/fotos`, `GET` y `DELETE /presupuestos/{id}/fotos/{foto}`.

**Confirmar** un presupuesto de muebles descuenta del depósito elegido, además de los
renglones sueltos, las placas, los metros de canto y los insumos calculados del despiece
(todo en la misma transacción: si falta algo no se descuenta nada). Al cancelar se
devuelve lo que salió según los movimientos de stock de la venta, no un recálculo.
El **plano de cortes** (`/presupuestos/{id}/cortes`) se imprime con una hoja A4 con el
despiece y los materiales y una hoja apaisada por placa, con la lista de piezas para tildar.

## Stock

- Tipos de producto: placa (se cuenta por placa), canto (por metro), herraje y otro (por unidad).
  El tipo no se cambia después de creado, porque los movimientos ya están en esa unidad.
- Movimientos: **ingreso** (compra; actualiza el costo del producto), **ajuste por conteo**
  (se carga lo contado y se registra la diferencia, con motivo obligatorio) y
  **traspaso** entre depósitos, aunque sean de distintas sucursales.
- El stock nunca queda negativo. La regla vive en `dominio/stock` como una
  `Operacion` que calcula el saldo nuevo a partir del actual; el repositorio la
  aplica con la fila de saldo bloqueada (`SELECT ... FOR UPDATE`), así dos
  movimientos al mismo tiempo no se pisan. Hay un test con 20 salidas simultáneas.
- Las claves foráneas no pasan por RLS, así que el repositorio confirma que el
  depósito es de la carpintería antes de usarlo.
- Stock bajo: total de todos los depósitos menor al stock mínimo del producto.

## Retazos

- Tabla `retazos` por depósito y placa, con estado disponible, usado o descartado,
  el pedido que lo generó (`pedido_origen_id`) y el que lo usó (`pedido_uso_id`).
- Se cargan a mano en Stock > Retazos (no pueden ser más grandes que la placa) y se descartan desde ahí.
- El presupuesto de muebles se calcula siempre con placas enteras, sin retazos.
  Al confirmar se descuentan el canto y los herrajes, el plan se guarda en
  `pedidos.materiales` y queda un **corte optimizado** pendiente por placa,
  optimizado con los retazos del depósito de salida.
- Precio de cada mueble: costo de materiales × 1,7 (`muebles.Recargo`), redondeado a
  la centena. Solo un mueble sin piezas ni herrajes lleva precio a mano.
- El optimizador suelto (Cortes) puede traer los retazos de una placa del stock o cargarlos a mano.

## Cortes optimizados

- Tabla `planes_corte` (estado pendiente, cortado o anulado) con la placa, el
  depósito, los parámetros, las piezas y el resultado en jsonb.
- Se crean desde el optimizador ("Guardar para cortar", con una placa del stock
  y un depósito) o al confirmar un presupuesto de muebles.
- "Se cortó" (permiso `cortes.confirmar`) hace todo en una transacción:
  - descuenta las placas enteras con un movimiento `corte`, que no es "venta", así
    cancelar la venta no las devuelve;
  - marca como usados los retazos del plan (si alguno ya no está, falla con 409 y
    hay que tocar "Recalcular");
  - guarda los sobrantes como retazos con `plan_origen_id`.
- Cancelar una venta anula sus cortes pendientes. Los que ya se cortaron quedan como están.
- Veta por pieza: sin veta, horizontal (a lo largo de la pieza) o vertical (a lo
  ancho, `veta_vertical`). La veta de la placa va a lo largo, así que una pieza
  vertical se coloca girada.

## Login y permisos

- **Email único en todo el sistema.** Al entrar, la tabla `cuentas_login` (sin
  RLS, solo email → usuario y carpintería) dice a qué carpintería pertenece el
  email. Un trigger la mantiene al día desde `usuarios`. La contraseña se lee
  después de `usuarios`, ya dentro de la carpintería.
- **Contraseñas** con bcrypt. Si el email no existe se compara igual contra un
  hash falso, para que la respuesta tarde lo mismo.
- **Sesiones** en la tabla `sesiones`: el navegador guarda un token aleatorio en
  una cookie `HttpOnly`, `Secure` y `SameSite=Lax`, y la base guarda solo su hash.
  Duran 30 días. Cerrar sesión borra la fila, así que se pueden revocar.
- **CSRF**: además de `SameSite=Lax`, todo pedido que cambia datos tiene que
  ser JSON, cosa que un formulario de otro sitio no puede mandar.
- **Límite de intentos**: 10 intentos fallidos cada 15 minutos por IP.
- **Permisos por sucursal**: la sesión guarda la sucursal elegida. Los permisos
  salen de los roles del usuario en esa sucursal más los roles sin sucursal.
  Cada endpoint declara el permiso que necesita (`requierePermiso("cortes.generar")`).
  La regla de qué permisos aplican en cada sucursal está en `dominio/acceso`.
- **Alta de carpinterías**: por ahora con `nandubay-admin crear-carpinteria`,
  que crea la carpintería, una sucursal, un depósito, los roles base y el admin.

## Frontend

- React + TypeScript + Vite, con componentes de **shadcn/ui** copiados en
  `frontend/src/components/ui`. Para agregar más se puede usar `npx shadcn add <componente>`.
- Paleta propia en tonos madera (`frontend/src/index.css`), con modo claro y oscuro.
- Router **wouter** (unos 2 KB) en vez de React Router, para mantener la app liviana.
- Cada pantalla se carga recién cuando se entra (`React.lazy`).
- Para gráficos de estadísticas usar una librería chica (uPlot o Chart.js), cargada solo en esa pantalla.
- Hoy la carga inicial es de unos 125 KB comprimidos de JavaScript y 11 KB de CSS.
