CREATE TABLE user_profile_images (
  id bigint NOT NULL PRIMARY KEY GENERATED BY DEFAULT AS IDENTITY,
  node integer NOT NULL DEFAULT 0,
  user_id varchar(255) NOT NULL,
  "position" integer NOT NULL DEFAULT 0,
  created_at timestamp with time zone
);
ALTER TABLE user_profile_images ADD CONSTRAINT uniq_user_profile_images_user_id_position UNIQUE (
  user_id, "position"
) DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE user_profile_images ADD CONSTRAINT fk_user_profile_images_user_id FOREIGN KEY (user_id)
  REFERENCES users(id);
