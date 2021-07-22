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

ALTER TABLE FederationInQuery
  ALTER query_id TYPE VARCHAR(50),
  ALTER server_addr TYPE VARCHAR(100),
  ALTER include_regions TYPE VARCHAR(5)[],
  ALTER exclude_regions TYPE VARCHAR(5)[],
  ALTER oidc_audience TYPE VARCHAR(1000),
  ALTER primary_cursor TYPE VARCHAR(100),
  ALTER revised_cursor TYPE VARCHAR(100);

DROP INDEX IF EXISTS federationinquery_last_timestamp;
DROP INDEX IF EXISTS federationinquery_only_local_provenance;
DROP INDEX IF EXISTS federationinquery_only_travelers;
DROP INDEX IF EXISTS federationinquery_last_revised_timestamp;
DROP INDEX IF EXISTS federationinquery_primary_cursor;
DROP INDEX IF EXISTS federationinquery_revised_cursor;

END;
