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

-- health_authority_id is nullable to preserve backwards compatibility.
ALTER TABLE exposure
	ADD COLUMN health_authority_id INT REFERENCES HealthAuthority(id),
	ADD COLUMN report_type VARCHAR(20) DEFAULT '' NOT NULL,
	ADD COLUMN days_since_symptom_onset INT,
	ADD COLUMN revised_report_type VARCHAR(20),
	ADD COLUMN revised_at TIMESTAMPTZ,
	ADD COLUMN revised_days_since_symptom_onset INT;

CREATE INDEX exposure_revised_at_idx ON exposure USING BRIN(revised_at);

END;
