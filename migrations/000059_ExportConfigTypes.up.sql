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

ALTER SEQUENCE exportconfig_config_id_seq AS BIGINT;
ALTER TABLE ExportConfig ALTER config_id TYPE BIGINT;
ALTER TABLE ExportConfig ALTER filename_root TYPE TEXT;
ALTER TABLE ExportConfig ALTER period_seconds TYPE BIGINT;
ALTER TABLE ExportConfig ALTER output_region TYPE TEXT;
ALTER TABLE ExportConfig ALTER bucket_name TYPE TEXT;
ALTER TABLE ExportConfig ALTER signature_info_ids TYPE BIGINT[];
ALTER TABLE ExportConfig ALTER input_regions TYPE TEXT[];
ALTER TABLE ExportConfig ALTER exclude_regions TYPE TEXT[];
ALTER TABLE ExportConfig ALTER max_records_override TYPE BIGINT;

CREATE INDEX exportconfig_filename_root ON ExportConfig(filename_root);
CREATE INDEX exportconfig_from_timestamp ON ExportConfig(from_timestamp);
CREATE INDEX exportconfig_thru_timestamp ON ExportConfig(thru_timestamp);
CREATE INDEX exportconfig_bucket_name ON ExportConfig(bucket_name);
CREATE INDEX exportconfig_signature_info_ids ON ExportConfig(signature_info_ids);
CREATE UNIQUE INDEX exportconfig_bucket_name_filename_root ON ExportConfig(LOWER(filename_root), LOWER(bucket_name));
CREATE INDEX exportconfig_include_travelers ON ExportConfig(include_travelers);
CREATE INDEX exportconfig_only_non_travelers ON ExportConfig(only_non_travelers);

DROP INDEX IF EXISTS uidx_bucket_name_filename_root;

END;
