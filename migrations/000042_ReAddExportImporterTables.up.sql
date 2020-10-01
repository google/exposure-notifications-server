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

-- Top level configuration for other key server export files
-- that should be imported into this server.
CREATE TABLE ExportImport(
    id SERIAL PRIMARY KEY,
    index_file VARCHAR(500) NOT NULL,
    export_root VARCHAR(500) NOT NULL,
    region VARCHAR(5) NOT NULL,
    from_timestamp TIMESTAMPTZ NOT NULL,
	thru_timestamp TIMESTAMPTZ
);

CREATE INDEX export_import_from_timestamp ON ExportImport USING BRIN(from_timestamp);
CREATE INDEX export_import_thru_timestamp ON ExportImport USING BRIN(thru_timestamp);

-- Public key information for an import config.
CREATE TABLE ImportFilePublicKey(
    export_import_id INT NOT NULL REFERENCES ExportImport(id),
    key_id VARCHAR(50) NOT NULL,
    key_version VARCHAR(50) NOT NULL,
    public_key VARCHAR(500) NOT NULL,
    from_timestamp TIMESTAMPTZ NOT NULL,
	thru_timestamp TIMESTAMPTZ,
    PRIMARY KEY(export_import_id, key_id, key_version)
);


CREATE INDEX import_file_public_key_from ON ImportFilePublicKey USING BRIN(from_timestamp);
CREATE INDEX import_file_public_key_thru ON ImportFilePublicKey USING BRIN(thru_timestamp);

-- Individual .zip export files that are being imported into this serever.
CREATE TYPE ImportFileStatus AS ENUM ('OPEN', 'PENDING', 'COMPLETE', 'FAILED');
CREATE TABLE ImportFile (
    id SERIAL PRIMARY KEY,
    export_import_id INT NOT NULL REFERENCES ExportImport(id),
    zip_filename VARCHAR(500) NOT NULL,
    discovered_at TIMESTAMPTZ NOT NULL,
    processed_at TIMESTAMPTZ,
    status ImportFileStatus NOT NULL DEFAULT 'OPEN' 
);

CREATE INDEX import_file_status_processed ON ImportFile (status, processed_at);

-- Allows us to link where an imported TEK came from
-- and marks when we saw the revision as well.
ALTER TABLE Exposure
    ADD COLUMN export_import_id INT,
    ADD COLUMN import_file_id INT,
    ADD COLUMN revised_import_file_id INT;


END;
