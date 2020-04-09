##

CREATE TABLE infection(
  id bigserial primary key,
  diagnosis_key  bytea not null,
  app_package_name varchar,
  federation_sync_id bigint,
  country character[2],
  receive_time timestamp not null);

CREATE INDEX countries ON infection (country, receive_time);

CREATE INDEX times ON infection (receive_time);
