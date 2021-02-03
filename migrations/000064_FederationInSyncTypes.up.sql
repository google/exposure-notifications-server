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

ALTER SEQUENCE federationsync_sync_id_seq RENAME TO federationinsync_sync_id_seq;
ALTER SEQUENCE federationinsync_sync_id_seq AS BIGINT;
ALTER TABLE FederationInSync
  ALTER sync_id TYPE BIGINT,
  ALTER query_id TYPE TEXT,
  ALTER insertions TYPE BIGINT;

CREATE INDEX federationinsync_query_id ON FederationInSync(query_id);
CREATE INDEX federationinsync_started ON FederationInSync(started);
CREATE INDEX federationinsync_completed ON FederationInSync(completed);
CREATE INDEX federationinsync_max_timestamp ON FederationInSync(max_timestamp);
CREATE INDEX federationinsync_max_revised_timestamp ON FederationInSync(max_revised_timestamp);

END;
