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

ALTER TABLE AuthorizedApp RENAME COLUMN safetynet_apk_digest TO apk_digest;
ALTER TABLE AuthorizedApp RENAME COLUMN safetynet_cts_profile_match TO cts_profile_match;
ALTER TABLE AuthorizedApp RENAME COLUMN safetynet_basic_integrity TO basic_integrity;
ALTER TABLE AuthorizedApp RENAME COLUMN safetynet_past_seconds TO allowed_past_seconds;
ALTER TABLE AuthorizedApp RENAME COLUMN safetynet_future_seconds TO allowed_future_seconds;
ALTER TABLE AuthorizedApp RENAME COLUMN devicecheck_team_id_secret TO ios_devicecheck_team_id_secret;
ALTER TABLE AuthorizedApp RENAME COLUMN devicecheck_key_id_secret TO ios_devicecheck_key_id_secret;
ALTER TABLE AuthorizedApp RENAME COLUMN devicecheck_private_key_secret TO ios_devicecheck_private_key_secret;
ALTER TABLE AuthorizedApp ADD COLUMN all_regions bool DEFAULT false;
CREATE INDEX authorized_app_app_package_name_idx ON AuthorizedApp(app_package_name);

END;
