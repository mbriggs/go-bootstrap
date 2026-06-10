-- Session store for alexedwards/scs (pgxstore). The schema is fixed by the
-- store: https://github.com/alexedwards/scs/tree/master/pgxstore

-- +goose Up
CREATE TABLE sessions (
  token text PRIMARY KEY,
  data bytea NOT NULL,
  expiry timestamptz NOT NULL
);

CREATE INDEX sessions_expiry_idx ON sessions (expiry);

-- +goose Down
DROP TABLE sessions;
