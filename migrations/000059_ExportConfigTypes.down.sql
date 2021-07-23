-- Copyright 2021 the Exposure Notification Server authors
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

CREATE INDEX uidx_bucket_name_filename_root ON exportconfig(LOWER(bucket_name), LOWER(filename_root));

ALTER SEQUENCE exportconfig_config_id_seq AS INT;
ALTER TABLE ExportConfig
  ALTER config_id TYPE INT,
  ALTER filename_root TYPE VARCHAR(100),
  ALTER period_seconds TYPE INT,
  ALTER output_region TYPE VARCHAR(5),
  ALTER bucket_name TYPE VARCHAR(64),
  ALTER signature_info_ids TYPE INT[],
  ALTER input_regions TYPE VARCHAR(5)[],
  ALTER exclude_regions TYPE VARCHAR(5)[],
  ALTER max_records_override TYPE INT;

DROP INDEX IF EXISTS exportconfig_filename_root ON ExportConfig(filename_root);
DROP INDEX IF EXISTS exportconfig_from_timestamp ON ExportConfig(from_timestamp);
DROP INDEX IF EXISTS exportconfig_thru_timestamp ON ExportConfig(thru_timestamp);
DROP INDEX IF EXISTS exportconfig_bucket_name ON ExportConfig(bucket_name);
DROP INDEX IF EXISTS exportconfig_signature_info_ids ON ExportConfig(signature_info_ids);
DROP INDEX IF EXISTS exportconfig_bucket_name_filename_root ON ExportConfig(filename_root, bucket_name);
DROP INDEX IF EXISTS exportconfig_include_travelers ON ExportConfig(include_travelers);
DROP INDEX IF EXISTS exportconfig_only_non_travelers ON ExportConfig(only_non_travelers);

END;
