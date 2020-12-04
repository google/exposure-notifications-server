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

CREATE TABLE HealthAuthorityStatsRelease(
    health_authority_id INT NOT NULL REFERENCES HealthAuthority(id),
    not_before TIMESTAMPTZ NOT NULL
);

CREATE TABLE HealthAuthorityStats(
    health_authority_id INT NOT NULL REFERENCES HealthAuthority(id) ON DELETE CASCADE,
    -- UTC hour
    hour TIMESTAMPTZ NOT NULL, 
    publish INT NOT NULL DEFAULT 0,
    teks INT NOT NULL DEFAULT 0,
    revisions INT NOT NULL DEFAULT 0,
    -- Age of the oldest TEKs from an individual publish request. Index is number of days.
    oldest_tek_days INT [],    
    -- Symptom onset to upload ranges, index is number of days.
    onset_age_days INT [],
    missing_onset INT DEFAULT 0
);

CREATE UNIQUE INDEX
    idx_health_authority_stats_hour ON HealthAuthorityStats (health_authority_id, hour);

END;
