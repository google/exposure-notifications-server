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

DROP INDEX exposure_revised_at_idx;

ALTER TABLE exposure
  DROP COLUMN health_authority_id,
	DROP COLUMN report_type,
	DROP COLUMN days_since_symptom_onset,
	DROP COLUMN revised_report_type,
	DROP COLUMN revised_at,
  DROP COLUMN revised_days_since_symptom_onset;

END;
