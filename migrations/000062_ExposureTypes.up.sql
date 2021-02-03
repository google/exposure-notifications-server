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

ALTER TABLE Exposure ALTER exposure_key TYPE TEXT;
ALTER TABLE Exposure ALTER app_package_name TYPE TEXT;
ALTER TABLE Exposure ALTER transmission_risk TYPE BIGINT;
ALTER TABLE Exposure ALTER regions TYPE TEXT[];
ALTER TABLE Exposure ALTER interval_number TYPE BIGINT;
ALTER TABLE Exposure ALTER interval_count TYPE BIGINT;
ALTER TABLE Exposure ALTER sync_id TYPE BIGINT;
ALTER TABLE Exposure ALTER health_authority_id TYPE BIGINT;
ALTER TABLE Exposure ALTER report_type TYPE TEXT;
ALTER TABLE Exposure ALTER days_since_symptom_onset TYPE BIGINT;
ALTER TABLE Exposure ALTER revised_report_type TYPE TEXT;
ALTER TABLE Exposure ALTER revised_days_since_symptom_onset TYPE BIGINT;
ALTER TABLE Exposure ALTER sync_query_id TYPE TEXT;
ALTER TABLE Exposure ALTER export_import_id TYPE BIGINT;
ALTER TABLE Exposure ALTER import_file_id TYPE BIGINT;
ALTER TABLE Exposure ALTER revised_import_file_id TYPE BIGINT;

ALTER INDEX infection_pkey RENAME TO exposure_pkey;

CREATE INDEX exposure_regions ON Exposure(regions);
CREATE INDEX exposure_created_at ON Exposure(created_at);
CREATE INDEX exposure_local_provenance ON Exposure(local_provenance);
CREATE INDEX exposure_revised_at ON Exposure(revised_at);
CREATE INDEX exposure_traveler ON Exposure(traveler);
CREATE INDEX exposure_sync_query_id ON Exposure(sync_query_id);
CREATE INDEX exposure_export_import_id ON Exposure(export_import_id);
CREATE INDEX exposure_import_file_id ON Exposure(import_file_id);
CREATE INDEX exposure_revised_import_file_id ON Exposure(revised_import_file_id);

DROP INDEX IF EXISTS exposure_created_at_idx;
DROP INDEX IF EXISTS exposure_revised_at_idx;

END;
