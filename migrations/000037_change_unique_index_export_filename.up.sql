-- Copyright 2020 the Exposure Notification Server authors
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


CREATE UNIQUE INDEX uidx_bucket_name_filename_root ON exportconfig (LOWER(bucket_name), LOWER(filename_root));

ALTER TABLE exportconfig DROP CONSTRAINT IF EXISTS filename_root_unique;
DROP INDEX IF EXISTS filename_root_unique;

END;
