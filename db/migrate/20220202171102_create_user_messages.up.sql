-- Вся система личных сообщений
CREATE TYPE dialogs_type AS ENUM (
  'system',
  'support',
  'user'
);

CREATE TABLE dialogs (
  id varchar(255) NOT NULL PRIMARY KEY,
  dialog_type dialogs_type NOT NULL DEFAULT 'system',
  last_message_id varchar(255) NOT NULL,
  deleted_at timestamp with time zone,
  created_at timestamp with time zone NOT NULL
);
CREATE UNIQUE INDEX index_dialogs_last_message_id ON dialogs(last_message_id);

CREATE TABLE user_dialogs (
  id varchar(255) NOT NULL PRIMARY KEY,
  dialog_id varchar(255) NOT NULL,
  user_id varchar(255) NOT NULL
);
ALTER TABLE user_dialogs ADD CONSTRAINT uniq_user_dialogs_dialog_id_user_id UNIQUE (dialog_id, user_id);

CREATE INDEX index_user_dialogs_user_id ON user_dialogs (user_id);
ALTER TABLE user_dialogs ADD CONSTRAINT fk_user_dialogs_users FOREIGN KEY (user_id)
  REFERENCES users(id);

CREATE TYPE user_messages_content_type AS ENUM (
  'text',
  'gift',
  'image',
  'video',
  'voice'
);

CREATE TABLE user_messages (
  id varchar(255) NOT NULL PRIMARY KEY,
  dialog_id varchar(255) NOT NULL,
  user_id varchar(255),
  read_at timestamp with time zone,
  "message" text NOT NULL,
  content_type user_messages_content_type NOT NULL DEFAULT 'text',
  created_at timestamp with time zone NOT NULL
);
ALTER TABLE user_messages ADD CONSTRAINT fk_user_messages_dialogs FOREIGN KEY (dialog_id)
  REFERENCES dialogs(id);
ALTER TABLE user_messages ADD CONSTRAINT fk_user_messages_users FOREIGN KEY (user_id)
  REFERENCES users(id);
CREATE INDEX index_user_messages_dialog_id ON user_messages (dialog_id);

CREATE TABLE user_messages_voices (
  id bigint NOT NULL PRIMARY KEY GENERATED BY DEFAULT AS IDENTITY,
  node integer NOT NULL DEFAULT 0,
  duration integer NOT NULL,
  message_id varchar(255) NOT NULL,
  created_at timestamp with time zone NOT NULL
);

CREATE UNIQUE INDEX index_user_messages_voices_message_id ON user_messages_voices (message_id);
ALTER TABLE user_messages_voices ADD CONSTRAINT
  fk_user_messages_voices_message_id FOREIGN KEY (message_id)
  REFERENCES user_messages(id);

CREATE TABLE user_messages_images (
  id bigint NOT NULL PRIMARY KEY GENERATED BY DEFAULT AS IDENTITY,
  node integer NOT NULL DEFAULT 0,
  created_at timestamp with time zone NOT NULL
);

CREATE TABLE user_messages_images_messages (
  id bigint NOT NULL PRIMARY KEY GENERATED BY DEFAULT AS IDENTITY,
  image_id bigint NOT NULL,
  message_id varchar(255) NOT NULL
);
ALTER TABLE user_messages_images_messages ADD CONSTRAINT
  uniq_user_messages_images_messages_image_message UNIQUE (message_id, image_id);
ALTER TABLE user_messages_images_messages ADD CONSTRAINT
  fk_user_messages_images_messages_image_id FOREIGN KEY (image_id)
  REFERENCES user_messages_images(id);
ALTER TABLE user_messages_images_messages ADD CONSTRAINT
  fk_user_messages_images_messages_message_id FOREIGN KEY (message_id)
  REFERENCES user_messages(id);

CREATE TABLE user_messages_videos (
  id bigint NOT NULL PRIMARY KEY GENERATED BY DEFAULT AS IDENTITY,
  node integer NOT NULL DEFAULT 0,
  created_at timestamp with time zone NOT NULL
);

CREATE TABLE user_messages_videos_messages (
  id bigint NOT NULL PRIMARY KEY GENERATED BY DEFAULT AS IDENTITY,
  video_id bigint NOT NULL,
  message_id varchar(255) NOT NULL
);
ALTER TABLE user_messages_videos_messages ADD CONSTRAINT
  uniq_user_messages_videos_messages_video_message UNIQUE (message_id, video_id);

ALTER TABLE user_messages_videos_messages ADD CONSTRAINT
  fk_user_messages_videos_messages_video_id FOREIGN KEY (video_id)
  REFERENCES user_messages_videos(id);

ALTER TABLE user_messages_videos_messages ADD CONSTRAINT
  fk_user_messages_videos_messages_message_id FOREIGN KEY (message_id)
  REFERENCES user_messages(id);

CREATE TABLE user_messages_gifts (
  id varchar(255) NOT NULL PRIMARY KEY,
  message_id varchar(255) NOT NULL,
  gift_id varchar(255) NOT NULL,
  created_at timestamp with time zone NOT NULL
);
CREATE UNIQUE INDEX index_user_messages_gifts_message_id ON user_messages_gifts (message_id);
ALTER TABLE user_messages_gifts ADD CONSTRAINT
  fk_user_messages_gifts_message_id FOREIGN KEY (message_id)
  REFERENCES user_messages(id);