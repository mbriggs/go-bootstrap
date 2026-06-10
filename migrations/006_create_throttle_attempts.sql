-- +goose Up
-- Failed signin / reset-request attempts, read as a sliding window by the
-- web package's throttle. Postgres-backed so the limit holds across
-- processes; rows prune opportunistically on insert.
CREATE TABLE throttle_attempts (
  key text NOT NULL,
  attempted_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX throttle_attempts_key_attempted_at ON throttle_attempts (key, attempted_at);
CREATE INDEX throttle_attempts_attempted_at ON throttle_attempts (attempted_at);

-- +goose Down
DROP TABLE throttle_attempts;
