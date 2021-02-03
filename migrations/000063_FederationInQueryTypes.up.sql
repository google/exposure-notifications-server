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

ALTER TABLE FederationInQuery
  ALTER query_id TYPE TEXT,
  ALTER server_addr TYPE TEXT,
  ALTER include_regions TYPE TEXT[],
  ALTER exclude_regions TYPE TEXT[],
  ALTER oidc_audience TYPE TEXT,
  ALTER primary_cursor TYPE TEXT,
  ALTER revised_cursor TYPE TEXT;

CREATE INDEX federationinquery_last_timestamp ON FederationInQuery(last_timestamp);
CREATE INDEX federationinquery_only_local_provenance ON FederationInQuery(only_local_provenance);
CREATE INDEX federationinquery_only_travelers ON FederationInQuery(only_travelers);
CREATE INDEX federationinquery_last_revised_timestamp ON FederationInQuery(last_revised_timestamp);
CREATE INDEX federationinquery_primary_cursor ON FederationInQuery(primary_cursor);
CREATE INDEX federationinquery_revised_cursor ON FederationInQuery(revised_cursor);

END;
