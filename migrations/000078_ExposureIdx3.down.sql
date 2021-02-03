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

CREATE INDEX exposure_created_at_idx ON Exposure(created_at);
CREATE INDEX exposure_revised_at_idx ON Exposure(revised_at);

DROP INDEX exposure_sync_query_id;
DROP INDEX exposure_export_import_id;
DROP INDEX exposure_import_file_id;
DROP INDEX exposure_revised_import_file_id;

END;
