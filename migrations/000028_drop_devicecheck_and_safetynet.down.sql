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

ALTER TABLE AuthorizedApp
  ADD COLUMN platform VARCHAR(10) NOT NULL DEFAULT '',
  ADD COLUMN safetynet_disabled BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN safetynet_apk_digest VARCHAR(64)[],
  ADD COLUMN safetynet_cts_profile_match BOOLEAN DEFAULT true,
  ADD COLUMN safetynet_basic_integrity BOOLEAN DEFAULT true,
  ADD COLUMN safetynet_past_seconds INTEGER,
  ADD COLUMN safetynet_future_seconds INTEGER,
  ADD COLUMN devicecheck_disabled BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN devicecheck_team_id VARCHAR(255),
  ADD COLUMN devicecheck_key_id VARCHAR(255),
  ADD COLUMN devicecheck_private_key_secret VARCHAR(255);

END;
