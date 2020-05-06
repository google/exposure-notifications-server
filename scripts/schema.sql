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


-- Changes to this file should be paired with migration additions. 
-- Whenever you update this please also update the version here to match.
CREATE TABLE public.schema_migrations
(
    version bigint NOT NULL,
    dirty boolean NOT NULL,
    CONSTRAINT schema_migrations_pkey PRIMARY KEY (version)
);

INSERT INTO public.schema_migrations (version, dirty) VALUES (1, false);

CREATE TABLE FederationQuery (
	query_id VARCHAR(50) PRIMARY KEY,
	server_addr VARCHAR(100) NOT NULL,
	include_regions VARCHAR(5) [],
	exclude_regions VARCHAR(5) [],
	last_timestamp TIMESTAMP
);

CREATE TABLE FederationSync (
	sync_id SERIAL PRIMARY KEY,
	query_id VARCHAR(50) NOT NULL REFERENCES FederationQuery (query_id),
	started TIMESTAMP NOT NULL,
	completed TIMESTAMP,
	insertions INT,
	max_timestamp TIMESTAMP
);

CREATE TABLE Exposure (
	exposure_key VARCHAR(30) PRIMARY KEY,
	transmission_risk INT NOT NULL,
	app_package_name VARCHAR(100),
	regions VARCHAR(5) [],
	interval_number INT NOT NULL,
	interval_count INT NOT NULL,
	created_at TIMESTAMP NOT NULL,
	local_provenance BOOLEAN NOT NULL,
	verification_authority_name VARCHAR(100),
	sync_id INT REFERENCES FederationSync (sync_id)
);

-- ExportConfig stores a list of batches to create on an ongoing basis. The /create-batches endpoint will iterate over this
-- table and create rows in the ExportBatchJob table.
CREATE TABLE ExportConfig (
	config_id SERIAL PRIMARY KEY,
	filename_root VARCHAR(100) NOT NULL,
	period_seconds INT NOT NULL,
	region VARCHAR(5) NOT NULL,
	from_timestamp TIMESTAMP NOT NULL,
	thru_timestamp TIMESTAMP
);

CREATE TYPE ExportBatchStatus AS ENUM ('OPEN', 'PENDING', 'COMPLETE', 'DELETED');
CREATE TABLE ExportBatch (
	batch_id SERIAL PRIMARY KEY,
	config_id INT NOT NULL REFERENCES ExportConfig(config_id),
	filename_root VARCHAR(100) NOT NULL,
	start_timestamp TIMESTAMP NOT NULL,
	end_timestamp TIMESTAMP NOT NULL,
	region VARCHAR(5) NOT NULL,
	status ExportBatchStatus NOT NULL DEFAULT 'OPEN',
	lease_expires TIMESTAMP
);

CREATE TABLE ExportFile (
	filename VARCHAR(200) PRIMARY KEY,
	batch_id INT REFERENCES ExportBatch(batch_id),
	region VARCHAR(5) NOT NULL,
	batch_num INT,
	batch_size INT,
	status VARCHAR(10)
);

CREATE TABLE Lock (
	lock_id VARCHAR(100) PRIMARY KEY,
	expires TIMESTAMP NOT NULL
);

CREATE OR REPLACE FUNCTION AcquireLock(VARCHAR(100), INT) RETURNS TIMESTAMP AS $$
	DECLARE
		nowT TIMESTAMP;
		expiresT TIMESTAMP;
	BEGIN
		nowT := CURRENT_TIMESTAMP;
		expiresT := nowT + '1 SECOND'::interval * $2;

		IF EXISTS (SELECT lock_id FROM Lock WHERE lock_id = $1 AND expires > nowT) THEN
			RETURN to_timestamp(0); -- Special value indicating no lock acquired.
		END IF;

		IF EXISTS (SELECT lock_id FROM Lock WHERE lock_id = $1) THEN
			UPDATE Lock SET expires = expiresT WHERE lock_id = $1;
			RETURN expiresT;
		END IF;

		INSERT INTO Lock (lock_id, expires) VALUES ($1, expiresT);
		RETURN expiresT;
	END
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION ReleaseLock(VARCHAR(100), TIMESTAMP) RETURNS BOOLEAN AS $$
	BEGIN
		IF NOT EXISTS (SELECT lock_id FROM Lock WHERE lock_id = $1 AND expires = $2) THEN
			RETURN FALSE; -- Another process acquired an expired lock
		END IF;

		DELETE FROM Lock WHERE lock_id = $1;
		RETURN TRUE;
	END
$$ LANGUAGE plpgsql;


CREATE TABLE APIConfig (
	app_package_name VARCHAR(1000) PRIMARY KEY,
	platform VARCHAR(10) NOT NULL,
	apk_digest VARCHAR(64),
	enforce_apk_digest BOOLEAN NOT NULL,
	cts_profile_match BOOLEAN NOT NULL,
	basic_integrity BOOLEAN NOT NULL,
	allowed_past_seconds INT,
	allowed_future_seconds INT,
	allowed_regions VARCHAR(5) [] NOT NULL,
	all_regions bool NOT NULL,
	bypass_safetynet bool NOT NULL
);



