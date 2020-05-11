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

-- Add a column for bucket_name of export configurations.

ALTER TABLE ExportConfig ADD COLUMN bucket_name VARCHAR(64) NOT NULL;
ALTER TABLE ExportBatch ADD COLUMN bucket_name VARCHAR(64) NOT NULL;
ALTER TABLE ExportFile ADD COLUMN bucket_name VARCHAR(64) NOT NULL;

-- Ensure filename_root is unique to avoid collisions if multiple configs
ALTER TABLE ExportConfig ADD CONSTRAINT filename_root_unique UNIQUE (filename_root);

END;
