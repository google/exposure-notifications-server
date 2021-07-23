-- Copyright 2020 the Exposure Notification Server authors
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

BEGIN;

-- This migration is destructive to any existing data.

CREATE TABLE SignatureInfo (
  id SERIAL PRIMARY KEY,
  signing_key VARCHAR(500) NOT NULL,
  app_package_name VARCHAR(1000) NOT NULL DEFAULT '',
  bundle_id VARCHAR(1000) NOT NULL DEFAULT '',
  signing_key_version VARCHAR(100) NOT NULL DEFAULT '',
  signing_key_id VARCHAR(50) NOT NULL DEFAULT '',
	thru_timestamp TIMESTAMPTZ
);

ALTER TABLE SignatureInfo SET (autovacuum_enabled = true);

ALTER TABLE ExportConfig
  ADD COLUMN signature_info_ids INT [],
  DROP COLUMN signing_key;

ALTER TABLE ExportBatch
  ADD COLUMN signature_info_ids INT [],
  DROP COLUMN signing_key;

END;
