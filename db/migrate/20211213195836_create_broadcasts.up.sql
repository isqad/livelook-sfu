CREATE TYPE broadcast_state AS ENUM (
  'initial',
  'running',
  'stopped',
  'errored'
);

CREATE TABLE broadcasts (
  id varchar(255) NOT NULL PRIMARY KEY,
  title varchar(255) NOT NULL,
  user_id varchar(255) NOT NULL,
  state broadcast_state NOT NULL DEFAULT 'initial',
  errors text NOT NULL DEFAULT '',
  created_at timestamp with time zone
);

CREATE INDEX index_broadcasts_user_id ON broadcasts (user_id);

ALTER TABLE broadcasts ADD CONSTRAINT fk_broadcasts_user_id FOREIGN KEY (user_id) REFERENCES users(id)
  ON DELETE CASCADE;

