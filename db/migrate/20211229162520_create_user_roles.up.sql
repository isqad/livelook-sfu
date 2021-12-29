CREATE TYPE user_role_type AS ENUM (
  'user',
  'admin'
);

CREATE TABLE user_roles (
  id varchar(255) NOT NULL PRIMARY KEY,
  name user_role_type NOT NULL DEFAULT 'user',
  user_id varchar(255) NOT NULL
);

CREATE UNIQUE INDEX index_user_roles_user_id_name ON user_roles (user_id, name);

ALTER TABLE user_roles ADD CONSTRAINT fk_user_roles_users FOREIGN KEY user_id REFERENCES users(id)
  ON DELETE CASCADE;
