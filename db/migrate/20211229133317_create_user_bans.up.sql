CREATE TYPE ban_type AS ENUM (
  'short',
  'long',
  'permanent'
);

CREATE TABLE user_bans (
  id varchar(255) NOT NULL PRIMARY KEY,
  user_id varchar(255) NOT NULL,
  "type" ban_type NOT NULL DEFAULT 'short',
  reason text NOT NULL,
  created_at timestamp with time zone
);

CREATE INDEX index_user_bans_user_id ON user_bans (user_id);

ALTER TABLE user_bans ADD CONSTRAINT fk_user_bans_user_id FOREIGN KEY (user_id) REFERENCES users(id)
  ON DELETE CASCADE;

