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

ALTER TABLE FederationQuery
	ALTER COLUMN last_timestamp type TIMESTAMP;

ALTER TABLE FederationSync
	ALTER COLUMN started type TIMESTAMP,
	ALTER COLUMN completed type TIMESTAMP,
	ALTER COLUMN max_timestamp type TIMESTAMP;

ALTER TABLE Exposure
	ALTER COLUMN created_at type TIMESTAMP;

ALTER TABLE ExportConfig
	ALTER COLUMN from_timestamp type TIMESTAMP,
	ALTER COLUMN thru_timestamp type TIMESTAMP;

ALTER TABLE ExportBatch
	ALTER COLUMN start_timestamp type TIMESTAMP,
	ALTER COLUMN end_timestamp type TIMESTAMP,
	ALTER COLUMN lease_expires type TIMESTAMP;

ALTER TABLE Lock
	ALTER COLUMN expires type TIMESTAMP;
