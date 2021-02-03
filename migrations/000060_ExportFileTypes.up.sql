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

ALTER TABLE ExportFile ALTER filename TYPE TEXT;
ALTER TABLE ExportFile ALTER batch_id TYPE BIGINT;
ALTER TABLE ExportFile ALTER output_region TYPE TEXT;
ALTER TABLE ExportFile ALTER batch_num TYPE BIGINT;
ALTER TABLE ExportFile ALTER batch_size TYPE BIGINT;
ALTER TABLE ExportFile ALTER status TYPE TEXT;
ALTER TABLE ExportFile ALTER bucket_name TYPE TEXT;
ALTER TABLE ExportFile ALTER input_regions TYPE TEXT[];
ALTER TABLE ExportFile ALTER exclude_regions TYPE TEXT[];

CREATE INDEX exportfile_batch_id ON ExportFile(batch_id);
CREATE INDEX exportfile_status ON ExportFile(status);
CREATE INDEX exportfile_bucket_name ON ExportFile(bucket_name);
CREATE INDEX exportfile_include_travelers ON ExportFile(include_travelers);
CREATE INDEX exportfile_only_non_travelers ON ExportFile(only_non_travelers);

END;
