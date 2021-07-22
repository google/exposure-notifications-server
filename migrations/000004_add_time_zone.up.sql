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

-- Change every TIMESTAMP type to TIMESTAMPTZ, preserving data.

-- This migration does not need to be run in a transaction.
-- Each ALTER TABLE happens atomically, and is idempotent.

ALTER TABLE FederationQuery
	ALTER COLUMN last_timestamp type TIMESTAMPTZ USING last_timestamp AT TIME ZONE 'UTC';

ALTER TABLE FederationSync
	ALTER COLUMN started type TIMESTAMPTZ USING started AT TIME ZONE 'UTC',
	ALTER COLUMN completed type TIMESTAMPTZ USING completed AT TIME ZONE 'UTC',
	ALTER COLUMN max_timestamp type TIMESTAMPTZ USING max_timestamp AT TIME ZONE 'UTC';

ALTER TABLE Exposure
	ALTER COLUMN created_at type TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC';

ALTER TABLE ExportConfig
	ALTER COLUMN from_timestamp type TIMESTAMPTZ USING from_timestamp AT TIME ZONE 'UTC',
	ALTER COLUMN thru_timestamp type TIMESTAMPTZ USING thru_timestamp AT TIME ZONE 'UTC';

ALTER TABLE ExportBatch
	ALTER COLUMN start_timestamp type TIMESTAMPTZ USING start_timestamp AT TIME ZONE 'UTC',
	ALTER COLUMN end_timestamp type TIMESTAMPTZ USING end_timestamp AT TIME ZONE 'UTC',
	ALTER COLUMN lease_expires type TIMESTAMPTZ USING lease_expires AT TIME ZONE 'UTC';

ALTER TABLE Lock
	ALTER COLUMN expires type TIMESTAMPTZ USING expires AT TIME ZONE 'UTC';
