-- Copyright 2020 Google LLC
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

ALTER TABLE federationinquery
    ADD COLUMN only_local_provenance BOOL DEFAULT TRUE,
    ADD COLUMN only_travelers BOOL DEFAULT FALSE,
    ADD COLUMN last_revised_timestamp TIMESTAMPTZ,
    ADD COLUMN primary_cursor VARCHAR(100),
    ADD COLUMN revised_cursor VARCHAR(100);

ALTER TABLE federationinquery
    ALTER COLUMN only_local_provenance SET NOT NULL,
    ALTER COLUMN only_travelers SET NOT NULL;    

ALTER TABLE exposure
    ADD COLUMN sync_query_id VARCHAR(50);

ALTER TABLE federationinsync
    ADD COLUMN max_revised_timestamp TIMESTAMPTZ;

END;
