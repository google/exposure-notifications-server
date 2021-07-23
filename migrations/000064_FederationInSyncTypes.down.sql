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

ALTER SEQUENCE federationinsync_sync_id_seq RENAME TO federationsync_sync_id_seq;
ALTER SEQUENCE federationsync_sync_id_seq AS INT;
ALTER TABLE FederationInSync
  ALTER sync_id TYPE INT,
  ALTER query_id TYPE VARCHAR(50),
  ALTER insertions TYPE INT;

DROP INDEX IF EXISTS federationinsync_query_id;
DROP INDEX IF EXISTS federationinsync_started;
DROP INDEX IF EXISTS federationinsync_completed;
DROP INDEX IF EXISTS federationinsync_max_timestamp;
DROP INDEX IF EXISTS federationinsync_max_revised_timestamp;

END;
