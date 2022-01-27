CREATE TABLE user_clients (
  id varchar(255) NOT NULL PRIMARY KEY,
  user_id varchar(255) NOT NULL,
  user_agent text NOT NULL,
  ip varchar(255) NOT NULL,
  created_at timestamp with time zone NOT NULL
);

COMMENT ON TABLE user_clients IS 'Tracks user agents of users';

ALTER TABLE user_clients ADD CONSTRAINT uniq_user_clients_user_agent_ip UNIQUE
  (user_id, ip, user_agent);

ALTER TABLE user_clients ADD CONSTRAINT fk_user_clients_users FOREIGN KEY (user_id)
  REFERENCES users(id);
