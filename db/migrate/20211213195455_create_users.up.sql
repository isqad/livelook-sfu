CREATE TABLE users (
  id varchar(255) NOT NULL PRIMARY KEY,
  email varchar(1024) NOT NULL,
  password varchar(1024) NOT NULL,
  "name" varchar(255) NOT NULL,
  confirm_selector varchar(1024),
  confirm_verifier varchar(1024),
  confirmed boolean NOT NULL DEFAULT false,
  created_at timestamp with time zone
);

CREATE UNIQUE INDEX index_users_email ON users (lower(email));
