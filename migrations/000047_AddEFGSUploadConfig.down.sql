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

BEGIN;

ALTER TABLE ExportConfig
    DROP COLUMN efgs_export,
    DROP COLUMN efgs_upload_host,
    DROP COLUMN efgs_mtls_cert_secret,
    DROP COLUMN efgs_mtls_key_secret,
    DROP COLUMN efgs_signing_cert_secret,
    DROP COLUMN efgs_signing_key_secret;

ALTER TABLE ExportBatch
    DROP COLUMN efgs_status exportbatchstatus;

END;
