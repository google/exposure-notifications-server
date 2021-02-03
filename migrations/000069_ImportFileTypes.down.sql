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

ALTER SEQUENCE importfile_id_seq AS INT;
ALTER TABLE ImportFile
  ALTER id TYPE INT,
  ALTER export_import_id TYPE INT,
  ALTER zip_filename TYPE VARCHAR(500),
  ALTER retries TYPE INT;

DROP INDEX IF EXISTS importfile_discovered_at;
DROP INDEX IF EXISTS importfile_status;

END;
