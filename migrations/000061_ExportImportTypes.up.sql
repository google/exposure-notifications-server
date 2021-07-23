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

ALTER SEQUENCE exportimport_id_seq AS BIGINT;
ALTER TABLE ExportImport
  ALTER id TYPE BIGINT,
  ALTER index_file TYPE TEXT,
  ALTER export_root TYPE TEXT,
  ALTER region TYPE TEXT;

CREATE INDEX exportimport_from_timestamp ON ExportImport(from_timestamp);
CREATE INDEX exportimport_thru_timestamp ON ExportImport(thru_timestamp);

DROP INDEX IF EXISTS export_import_from_timestamp;
DROP INDEX IF EXISTS export_import_thru_timestamp;

END;
