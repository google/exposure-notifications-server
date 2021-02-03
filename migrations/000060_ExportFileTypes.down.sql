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

ALTER TABLE ExportFile
  ALTER filename TYPE VARCHAR(100),
  ALTER batch_id TYPE INT,
  ALTER output_region TYPE VARCHAR(5),
  ALTER batch_num TYPE INT,
  ALTER batch_size TYPE INT,
  ALTER status TYPE VARCHAR(10),
  ALTER bucket_name TYPE VARCHAR(64),
  ALTER input_regions TYPE VARCHAR(5)[],
  ALTER exclude_regions TYPE VARCHAR(5)[];

DROP INDEX IF EXISTS exportfile_batch_id;
DROP INDEX IF EXISTS exportfile_status;
DROP INDEX IF EXISTS exportfile_bucket_name;
DROP INDEX IF EXISTS exportfile_include_travelers;
DROP INDEX IF EXISTS exportfile_only_non_travelers;

END;
