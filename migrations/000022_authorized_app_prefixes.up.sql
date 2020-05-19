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

-- Update the semantic meaning of all_regions
UPDATE AuthorizedApp SET allowed_regions = ARRAY[]::VARCHAR[]
WHERE all_regions = true;

ALTER TABLE AuthorizedApp RENAME COLUMN apk_digest TO safetynet_apk_digest;
ALTER TABLE AuthorizedApp RENAME COLUMN cts_profile_match TO safetynet_cts_profile_match;
ALTER TABLE AuthorizedApp RENAME COLUMN basic_integrity TO safetynet_basic_integrity;
ALTER TABLE AuthorizedApp RENAME COLUMN allowed_past_seconds TO safetynet_past_seconds;
ALTER TABLE AuthorizedApp RENAME COLUMN allowed_future_seconds TO safetynet_future_seconds;
ALTER TABLE AuthorizedApp RENAME COLUMN ios_devicecheck_team_id_secret TO devicecheck_team_id_secret;
ALTER TABLE AuthorizedApp RENAME COLUMN ios_devicecheck_key_id_secret TO devicecheck_key_id_secret;
ALTER TABLE AuthorizedApp RENAME COLUMN ios_devicecheck_private_key_secret TO devicecheck_private_key_secret;
ALTER TABLE AuthorizedApp DROP COLUMN all_regions;
DROP INDEX authorized_app_app_package_name_idx;

END;
