-- Copyright 2021 the Exposure Notification Server authors
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

ALTER TABLE Exposure
  ALTER exposure_key TYPE VARCHAR(30),
  ALTER app_package_name TYPE VARCHAR(100),
  ALTER transmission_risk TYPE INT,
  ALTER regions TYPE VARCHAR(5)[],
  ALTER interval_number TYPE INT,
  ALTER interval_count TYPE INT,
  ALTER sync_id TYPE INT,
  ALTER health_authority_id TYPE INT,
  ALTER report_type TYPE VARCHAR(20),
  ALTER days_since_symptom_onset TYPE INT,
  ALTER revised_report_type TYPE VARCHAR(20),
  ALTER revised_days_since_symptom_onset TYPE INT,
  ALTER sync_query_id TYPE VARCHAR(50),
  ALTER export_import_id TYPE INT,
  ALTER import_file_id TYPE INT,
  ALTER revised_import_file_id TYPE INT;

ALTER INDEX exposure_pkey RENAME TO infection_pkey;

END;
