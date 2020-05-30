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

CREATE TABLE HealthAuthority(
	id SERIAL PRIMARY KEY,
	iss VARCHAR(200) NOT NULL,
	aud VARCHAR(200) NOT NULL,
  name VARCHAR(200) NOT NULL,
  CONSTRAINT uniqueiss UNIQUE(iss)
);

CREATE TABLE HealthAuthorityKey(
	health_authority_id INT NOT NULL REFERENCES HealthAuthority(id),
	version varchar(100) NOT NULL,
	from_timestamp TIMESTAMPTZ NOT NULL,
	thru_timestamp TIMESTAMPTZ,
	public_key VARCHAR(500),
	PRIMARY KEY(health_authority_id, version)
);

ALTER TABLE AuthorizedApp
  ADD COLUMN health_authority_ids int[];


END;
