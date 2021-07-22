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

ALTER TABLE federationinquery
    DROP COLUMN only_local_provenance,
    DROP COLUMN only_travelers,
    DROP COLUMN last_revised_timestamp,
    DROP COLUMN primary_cursor,
    DROP COLUMN revised_cursor;

ALTER TABLE exposure
    DROP COLUMN sync_query_id;

ALTER TABLE federationinsync
    DROP COLUMN max_revised_timestamp;

END;
