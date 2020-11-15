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
    not_before TIMESTAMPTZ NOT NULL,
    -- timeone name, i.e. America/New_York
    timezone TEXT,
    -- last time that the timezone was changed - this locks the PHA out from changing again for configured time duration.
    last_timezone_change TIMESTAMPTZ NOT NULL
);

CREATE TABLE HealthAuthorityStats(
    health_authority_id INT NOT NULL REFERENCES HealthAuthority(id),
    hour TIMESTAMPTZ NOT NULL,
    publish INT NOT NULL DEFAULT 0,
    teks INT NOT NULL DEFAULT 0,
    revisions INT NOT NULL DEFAULT 0,
    -- Age of the oldest TEKs from an individual publish request.
    oldest_tek_0_days INT NOT NULL DEFAULT 0,
    oldest_tek_1_days INT NOT NULL DEFAULT 0,
    oldest_tek_2_days INT NOT NULL DEFAULT 0,
    oldest_tek_3_days INT NOT NULL DEFAULT 0,
    oldest_tek_4_days INT NOT NULL DEFAULT 0,
    oldest_tek_5_days INT NOT NULL DEFAULT 0,
    oldest_tek_6_days INT NOT NULL DEFAULT 0,
    oldest_tek_7_days INT NOT NULL DEFAULT 0,
    oldest_tek_8_days INT NOT NULL DEFAULT 0,
    oldest_tek_9_days INT NOT NULL DEFAULT 0,
    oldest_tek_10_days INT NOT NULL DEFAULT 0,
    oldest_tek_11_days INT NOT NULL DEFAULT 0,
    oldest_tek_12_days INT NOT NULL DEFAULT 0,
    oldest_tek_13_days INT NOT NULL DEFAULT 0,
    oldest_tek_14_days INT NOT NULL DEFAULT 0,
    oldest_tek_gt_14_days INT NOT NULL DEFAULT 0,
    -- Symptom onset to upload ranges
    onset_age_0_days INT NOT NULL DEFAULT 0,
    onset_age_1_days INT NOT NULL DEFAULT 0,
    onset_age_2_days INT NOT NULL DEFAULT 0,
    onset_age_3_days INT NOT NULL DEFAULT 0,
    onset_age_4_days INT NOT NULL DEFAULT 0,
    onset_age_5_days INT NOT NULL DEFAULT 0,
    onset_age_6_days INT NOT NULL DEFAULT 0,
    onset_age_7_days INT NOT NULL DEFAULT 0,
    onset_age_8_days INT NOT NULL DEFAULT 0,
    onset_age_9_days INT NOT NULL DEFAULT 0,
    onset_age_10_days INT NOT NULL DEFAULT 0,
    onset_age_11_days INT NOT NULL DEFAULT 0,
    onset_age_12_days INT NOT NULL DEFAULT 0,
    onset_age_13_days INT NOT NULL DEFAULT 0,
    onset_age_14_days INT NOT NULL DEFAULT 0,
    onset_age_gt_14_days INT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX
    idx_health_authority_stats_hour ON HealthAuthorityStats (health_authority_id, hour);

END;
