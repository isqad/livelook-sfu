DROP INDEX index_users_uid;
ALTER TABLE users ADD CONSTRAINT uniq_users_uid UNIQUE ("uid");
