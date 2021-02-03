-- Copyright 2021 Google LLC
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

ALTER SEQUENCE exportconfig_batch_id_seq AS INT;
ALTER TABLE ExportConfig ALTER batch_id TYPE INT;
ALTER TABLE ExportBatch ALTER COLUMN config_id TYPE INT;
ALTER TABLE ExportBatch ALTER COLUMN filename_root TYPE VARCHAR(100);
ALTER TABLE ExportBatch ALTER COLUMN output_region TYPE VARCHAR(5);
ALTER TABLE ExportBatch ALTER COLUMN bucket_name TYPE VARCHAR(64);
ALTER TABLE ExportBatch ALTER COLUMN signature_info_ids TYPE INT[];
ALTER TABLE ExportBatch ALTER COLUMN input_regions TYPE VARCHAR(5)[];
ALTER TABLE ExportBatch ALTER COLUMN exclude_regions TYPE VARCHAR(5)[];
ALTER TABLE ExportBatch ALTER COLUMN max_records_override TYPE INT;

DROP INDEX IF EXISTS exportbatch_config_id;
DROP INDEX IF EXISTS exportbatch_status;
DROP INDEX IF EXISTS exportbatch_start_timestamp;
DROP INDEX IF EXISTS exportbatch_end_timestamp;

END;
