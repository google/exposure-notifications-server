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

CREATE TABLE FederationQuery (
	query_id VARCHAR(50) PRIMARY KEY,
	server_addr VARCHAR(100) NOT NULL,
	include_regions VARCHAR(5) [],
	exclude_regions VARCHAR(5) [],
	last_timestamp TIMESTAMP
);

CREATE TABLE FederationSync (
	sync_id VARCHAR(100) PRIMARY KEY,
	query_id VARCHAR(50) NOT NULL,
	started TIMESTAMP NOT NULL,
	completed TIMESTAMP,
	insertions INT,
	max_timestamp TIMESTAMP,
	FOREIGN KEY (query_id) REFERENCES FederationQuery (query_id)
);

CREATE TABLE Infection (
	exposure_key VARCHAR(30) PRIMARY KEY,
	diagnosis_status INT NOT NULL,
	app_package_name VARCHAR(100),
	regions VARCHAR(5) [],
	interval_number INT NOT NULL,
	interval_count INT NOT NULL,
	created_at TIMESTAMP NOT NULL,
	local_provenance BOOLEAN NOT NULL,
	verification_authority_name VARCHAR(100),
	sync_id VARCHAR(100)  -- This could be a foreign key to FederationSync, but it's more difficult to handle nullable strings in Go, and it seems like unnecessary overhead.
);

CREATE TABLE Lock (
	lock_id VARCHAR(100) PRIMARY KEY,
	expires TIMESTAMP NOT NULL
);

CREATE USER apollo WITH password 'mypassword';
CREATE ROLE apollo_service;
GRANT SELECT, INSERT, UPDATE, DELETE ON Lock, Infection, FederationSync, FederationQuery TO apollo_service;
GRANT apollo_service TO apollo;
