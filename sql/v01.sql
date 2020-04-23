-- Copyright 2020 Google LLC
--
-- Licensed under the Apache License, Version 2.0 (the "License");
-- you may not use this file except in compliance with the License.
-- You may obtain a copy of the License at
--
--      http://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing, software
-- distributed under the License is distributed on an "AS IS" BASIS,
-- WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
-- See the License for the specific language governing permissions and
-- limitations under the License.

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
