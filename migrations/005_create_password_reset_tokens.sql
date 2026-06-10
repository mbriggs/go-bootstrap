-- +goose Up
CREATE TABLE password_reset_tokens (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  -- sha256 of the token; the cleartext only ever lives in the emailed link
  token_hash text NOT NULL,
  expires_at timestamptz NOT NULL,
  used_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX password_reset_tokens_token_hash_unique ON password_reset_tokens (token_hash);
CREATE INDEX password_reset_tokens_user_id ON password_reset_tokens (user_id);

-- +goose Down
DROP TABLE password_reset_tokens;
