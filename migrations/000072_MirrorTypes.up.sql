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

ALTER SEQUENCE mirror_id_seq AS BIGINT;
ALTER TABLE Mirror
  ALTER id TYPE BIGINT,
  ALTER index_file TYPE TEXT,
  ALTER export_root TYPE TEXT,
  ALTER cloud_storage_bucket TYPE TEXT,
  ALTER filename_root TYPE TEXT,
  ALTER filename_rewrite TYPE TEXT;

END;
