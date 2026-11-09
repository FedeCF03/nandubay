-- +goose Up
-- Login con email único en todo el sistema.

ALTER TABLE usuarios DROP CONSTRAINT usuarios_carpinteria_id_email_key;
CREATE UNIQUE INDEX usuarios_email_unico ON usuarios (lower(email));

-- Índice global email -> usuario. No tiene RLS porque al entrar todavía no se
-- sabe de qué carpintería es el usuario. No guarda la contraseña: esa se lee
-- de usuarios, ya dentro de la carpintería.
CREATE TABLE cuentas_login (
    email           text PRIMARY KEY,           -- siempre en minúsculas
    usuario_id      bigint NOT NULL UNIQUE REFERENCES usuarios(id) ON DELETE CASCADE,
    carpinteria_id  bigint NOT NULL REFERENCES carpinterias(id)
);

-- +goose StatementBegin
CREATE FUNCTION sincronizar_cuenta_login() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  INSERT INTO cuentas_login (email, usuario_id, carpinteria_id)
  VALUES (lower(NEW.email), NEW.id, NEW.carpinteria_id)
  ON CONFLICT (usuario_id) DO UPDATE
    SET email = EXCLUDED.email, carpinteria_id = EXCLUDED.carpinteria_id;
  RETURN NEW;
END $$;
-- +goose StatementEnd

CREATE TRIGGER usuarios_cuenta_login
AFTER INSERT OR UPDATE OF email, carpinteria_id ON usuarios
FOR EACH ROW EXECUTE FUNCTION sincronizar_cuenta_login();

-- Sesiones abiertas. Se guarda el hash del token, nunca el token.
CREATE TABLE sesiones (
    token_hash      text PRIMARY KEY,
    usuario_id      bigint NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    carpinteria_id  bigint NOT NULL REFERENCES carpinterias(id),
    sucursal_id     bigint REFERENCES sucursales(id),   -- sucursal elegida
    creada_en       timestamptz NOT NULL DEFAULT now(),
    usada_en        timestamptz NOT NULL DEFAULT now(),
    expira_en       timestamptz NOT NULL,
    ip              text,
    agente          text
);
CREATE INDEX ON sesiones (usuario_id);
CREATE INDEX ON sesiones (expira_en);

ALTER TABLE usuarios ADD COLUMN ultimo_ingreso timestamptz;

-- +goose Down
ALTER TABLE usuarios DROP COLUMN ultimo_ingreso;
DROP TABLE sesiones;
DROP TRIGGER usuarios_cuenta_login ON usuarios;
DROP FUNCTION sincronizar_cuenta_login();
DROP TABLE cuentas_login;
DROP INDEX usuarios_email_unico;
ALTER TABLE usuarios ADD CONSTRAINT usuarios_carpinteria_id_email_key UNIQUE (carpinteria_id, email);
