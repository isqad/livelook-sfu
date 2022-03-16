ALTER TABLE users ADD COLUMN email varchar(1024),
  ADD COLUMN "password" varchar(1024);

CREATE UNIQUE INDEX uniq_users_email ON users (lower(email)) WHERE email IS NOT NULL;
