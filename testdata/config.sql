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

INSERT INTO AuthorizedApp (
  app_package_name, platform, allowed_regions,
  safetynet_cts_profile_match, safetynet_basic_integrity, safetynet_past_seconds, safetynet_future_seconds,
  devicecheck_team_id_secret, devicecheck_key_id_secret, devicecheck_private_key_secret
) VALUES (
  'com.example.ios.app', 'ios', ARRAY[]::VARCHAR[],
  false, false, 60, 60,
  'projects/38554818207/secrets/ios-devicecheck-team-id/versions/1', 'projects/38554818207/secrets/ios-devicecheck-key-id/versions/1', 'projects/38554818207/secrets/ios-devicecheck-private-key/versions/1'
), (
  'com.example.android.app', 'android', ARRAY[]::VARCHAR[],
  false, false, 60, 60,
  NULL, NULL, NULL
);
