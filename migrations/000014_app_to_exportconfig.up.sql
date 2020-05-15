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

BEGIN;

ALTER TABLE ExportConfig ADD COLUMN app_package_name VARCHAR(1000) NOT NULL DEFAULT '';
ALTER TABLE ExportConfig ADD COLUMN bundle_id VARCHAR(1000) NOT NULL DEFAULT '';
ALTER TABLE ExportConfig ADD COLUMN signing_key_version VARCHAR(100) NOT NULL DEFAULT '';
ALTER TABLE ExportConfig ADD COLUMN signing_key_id VARCHAR(255) NOT NULL DEFAULT '';

ALTER TABLE ExportBatch ADD COLUMN app_package_name VARCHAR(1000) NOT NULL DEFAULT '';
ALTER TABLE ExportBatch ADD COLUMN bundle_id VARCHAR(1000) NOT NULL DEFAULT '';
ALTER TABLE ExportBatch ADD COLUMN signing_key_version VARCHAR(100) NOT NULL DEFAULT '';
ALTER TABLE ExportBatch ADD COLUMN signing_key_id VARCHAR(255) NOT NULL DEFAULT '';

END;
