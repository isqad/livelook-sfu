ALTER TABLE users RENAME TO admin_users;

CREATE TABLE users (
  id varchar(255) NOT NULL PRIMARY KEY,
  "uid" varchar(1024) NOT NULL,
  "name" varchar(255) NOT NULL DEFAULT 'anonymous',
  created_at timestamp with time zone
);

CREATE UNIQUE INDEX index_users_uid ON users (lower("uid"));
