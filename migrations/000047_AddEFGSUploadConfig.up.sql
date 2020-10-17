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

-- Allow for batches to be federated out. 
ALTER TABLE ExportBatch
    ADD COLUMN efgs_status exportbatchstatus DEFAULT 'OPEN';

-- Mark a specific export config as needing to be federated UP to EFGS.
ALTER TABLE ExportConfig
    ADD COLUMN efgs_export BOOLEAN DEFAULT false,
    ADD COLUMN efgs_upload_host VARCHAR(250) DEFAULT '',
    ADD COLUMN efgs_mtls_cert_secret VARCHAR(250) DEFAULT '',
    ADD COLUMN efgs_mtls_key_secret VARCHAR(250) DEFAULT '',
    ADD COLUMN efgs_signing_cert_secret VARCHAR(250) DEFAULT '',
    ADD COLUMN efgs_signing_key_secret VARCHAR(250) DEFAULT '';

ALTER TABLE ExportConfig
    ALTER COLUMN efgs_export SET NOT NULL,
    ALTER COLUMN efgs_upload_host SET NOT NULL,
    ALTER COLUMN efgs_mtls_cert_secret SET NOT NULL,
    ALTER COLUMN efgs_mtls_key_secret SET NOT NULL,
    ALTER COLUMN efgs_signing_cert_secret SET NOT NULL,
    ALTER COLUMN efgs_signing_key_secret SET NOT NULL;

END;
