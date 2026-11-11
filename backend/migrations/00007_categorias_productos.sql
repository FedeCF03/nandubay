-- +goose Up
-- Categorías de productos que arma cada carpintería (placas, bisagras, guías,
-- herrajes...). Son solo para ordenar y filtrar: el tipo del producto sigue
-- siendo el que decide la unidad y si se usa en los cortes.
CREATE TABLE categorias_producto (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    carpinteria_id  bigint NOT NULL REFERENCES carpinterias(id),
    nombre          text NOT NULL CHECK (btrim(nombre) <> ''),
    descripcion     text,
    creado_en       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (carpinteria_id, id)
);
CREATE UNIQUE INDEX categorias_producto_nombre ON categorias_producto (carpinteria_id, lower(nombre));

ALTER TABLE categorias_producto ENABLE ROW LEVEL SECURITY;
ALTER TABLE categorias_producto FORCE ROW LEVEL SECURITY;
CREATE POLICY aislamiento ON categorias_producto USING (carpinteria_id = app_carpinteria_id())
    WITH CHECK (carpinteria_id = app_carpinteria_id());

-- La clave foránea incluye la carpintería: así un producto no puede quedar
-- con la categoría de otra carpintería (las claves foráneas no pasan por RLS).
-- Al borrar la categoría, el producto queda sin categoría.
ALTER TABLE productos ADD COLUMN categoria_id bigint;
ALTER TABLE productos ADD CONSTRAINT productos_categoria_fk
    FOREIGN KEY (carpinteria_id, categoria_id) REFERENCES categorias_producto (carpinteria_id, id)
    ON DELETE SET NULL (categoria_id);
CREATE INDEX ON productos (categoria_id);

-- +goose Down
ALTER TABLE productos DROP COLUMN categoria_id;
DROP TABLE categorias_producto;
