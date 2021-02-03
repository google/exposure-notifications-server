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

ALTER SEQUENCE importfile_id_seq AS BIGINT;
ALTER TABLE ImportFile
  ALTER id TYPE BIGINT,
  ALTER export_import_id TYPE BIGINT,
  ALTER zip_filename TYPE TEXT,
  ALTER retries TYPE BIGINT;

CREATE INDEX importfile_discovered_at ON ImportFile(discovered_at);
CREATE INDEX importfile_status ON ImportFile(status);

END;
