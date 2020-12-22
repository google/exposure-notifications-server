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

CREATE TABLE HealthAuthorityStats(
    health_authority_id INT NOT NULL REFERENCES HealthAuthority(id) ON DELETE CASCADE,
    -- UTC hour
    hour TIMESTAMPTZ NOT NULL,
    -- 3 elements, android/iphone/unknown
    publish BIGINT [],
    -- number of TEKs provided
    teks BIGINT NOT NULL DEFAULT 0,
    -- number of TEKs that were revised
    revisions BIGINT NOT NULL DEFAULT 0,
    -- Age of the oldest TEKs from an individual publish request. Index is number of days. 0-14
    oldest_tek_days BIGINT [],    
    -- Symptom onset to upload ranges, index is number of days. 0-28.
    onset_age_days BIGINT [],
    -- Indicator of where the symptom onset was backfilled.
    missing_onset BIGINT DEFAULT 0
);

CREATE UNIQUE INDEX
    idx_health_authority_stats_hour ON HealthAuthorityStats (health_authority_id, hour);

END;
