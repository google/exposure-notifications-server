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

ALTER TABLE Exposure
  ALTER exposure_key TYPE TEXT,
  ALTER app_package_name TYPE TEXT,
  ALTER transmission_risk TYPE BIGINT,
  ALTER regions TYPE TEXT[],
  ALTER interval_number TYPE BIGINT,
  ALTER interval_count TYPE BIGINT,
  ALTER sync_id TYPE BIGINT,
  ALTER health_authority_id TYPE BIGINT,
  ALTER report_type TYPE TEXT,
  ALTER days_since_symptom_onset TYPE BIGINT,
  ALTER revised_report_type TYPE TEXT,
  ALTER revised_days_since_symptom_onset TYPE BIGINT,
  ALTER sync_query_id TYPE TEXT,
  ALTER export_import_id TYPE BIGINT,
  ALTER import_file_id TYPE BIGINT,
  ALTER revised_import_file_id TYPE BIGINT;

ALTER INDEX infection_pkey RENAME TO exposure_pkey;

END;
