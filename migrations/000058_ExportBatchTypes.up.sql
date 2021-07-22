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

ALTER SEQUENCE exportbatch_batch_id_seq AS BIGINT;
ALTER TABLE ExportBatch
  ALTER batch_id TYPE BIGINT,
  ALTER config_id TYPE BIGINT,
  ALTER filename_root TYPE TEXT,
  ALTER output_region TYPE TEXT,
  ALTER bucket_name TYPE TEXT,
  ALTER signature_info_ids TYPE BIGINT[],
  ALTER input_regions TYPE TEXT[],
  ALTER exclude_regions TYPE TEXT[],
  ALTER max_records_override TYPE BIGINT;

CREATE INDEX exportbatch_config_id ON ExportBatch(config_id);
CREATE INDEX exportbatch_status ON ExportBatch(status);
CREATE INDEX exportbatch_start_timestamp ON ExportBatch(start_timestamp);
CREATE INDEX exportbatch_end_timestamp ON ExportBatch(end_timestamp);

END;
