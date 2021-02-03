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

ALTER TABLE Exposure ALTER exposure_key TYPE VARCHAR(30);
ALTER TABLE Exposure ALTER app_package_name TYPE VARCHAR(100);
ALTER TABLE Exposure ALTER transmission_risk TYPE INT;
ALTER TABLE Exposure ALTER regions TYPE VARCHAR(5)[];
ALTER TABLE Exposure ALTER interval_number TYPE INT;
ALTER TABLE Exposure ALTER interval_count TYPE INT;
ALTER TABLE Exposure ALTER sync_id TYPE INT;
ALTER TABLE Exposure ALTER health_authority_id TYPE INT;
ALTER TABLE Exposure ALTER report_type TYPE VARCHAR(20);
ALTER TABLE Exposure ALTER days_since_symptom_onset TYPE INT;
ALTER TABLE Exposure ALTER revised_report_type TYPE VARCHAR(20);
ALTER TABLE Exposure ALTER revised_days_since_symptom_onset TYPE INT;
ALTER TABLE Exposure ALTER sync_query_id TYPE VARCHAR(50);
ALTER TABLE Exposure ALTER export_import_id TYPE INT;
ALTER TABLE Exposure ALTER import_file_id TYPE INT;
ALTER TABLE Exposure ALTER revised_import_file_id TYPE INT;

CREATE INDEX exposure_created_at_idx ON Exposure(created_at) USING BRIN;
CREATE INDEX exposure_revised_at_idx ON Exposure(revised_at) USING BRIN;

ALTER INDEX exposure_pkey RENAME TO infection_pkey;

DROP INDEX IF EXISTS exposure_regions;
DROP INDEX IF EXISTS exposure_created_at;
DROP INDEX IF EXISTS exposure_local_provenance;
DROP INDEX IF EXISTS exposure_revised_at;
DROP INDEX IF EXISTS exposure_traveler;
DROP INDEX IF EXISTS exposure_sync_query_id;
DROP INDEX IF EXISTS exposure_export_import_id;
DROP INDEX IF EXISTS exposure_import_file_id;
DROP INDEX IF EXISTS exposure_revised_import_file_id;

END;
