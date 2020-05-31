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

ALTER TABLE AuthorizedApp
  DROP COLUMN safetynet_disabled,
  DROP COLUMN safetynet_apk_digest,
  DROP COLUMN safetynet_cts_profile_match,
  DROP COLUMN safetynet_basic_integrity,
  DROP COLUMN safetynet_past_seconds,
  DROP COLUMN safetynet_future_seconds,
  DROP COLUMN devicecheck_disabled,
  DROP COLUMN devicecheck_team_id,
  DROP COLUMN devicecheck_key_id,
  DROP COLUMN devicecheck_private_key_secret;

END;
