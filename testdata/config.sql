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

-- Use this as a template to inject configurations for your publish API endpoint.

BEGIN;

INSERT INTO AuthorizedApp (
  app_package_name, platform, allowed_regions,
  devicecheck_disabled, devicecheck_team_id, devicecheck_key_id, devicecheck_private_key_secret
) VALUES (
  'com.example.ios.app', 'ios', '{}',
  false, 'ABCD1234', 'DEFG5678', 'projects/12345/secrets/ios-devicecheck-private-key/versions/1'
);

INSERT INTO AuthorizedApp (
  app_package_name, platform, allowed_regions,
  safetynet_disabled, safetynet_cts_profile_match, safetynet_basic_integrity, safetynet_past_seconds, safetynet_future_seconds
) VALUES (
  'com.example.android.app', 'android', '{}',
  false, false, false, 60, 60
);

END;
