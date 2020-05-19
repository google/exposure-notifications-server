
-- Use this as a template to inject configurations for your publish API endpoint.

INSERT INTO authorizedapp (app_package_name, platform, cts_profile_match, basic_integrity, allowed_past_seconds, allowed_future_seconds, allowed_regions, all_regions, ios_devicecheck_team_id_secret, ios_devicecheck_key_id_secret, ios_devicecheck_private_key_secret) VALUES
  ('com.example.ios.app', 'ios', false, false, 60, 60, ARRAY ['GB','US'], false, 'projects/38554818207/secrets/ios-devicecheck-team-id/versions/1', 'projects/38554818207/secrets/ios-devicecheck-key-id/versions/1', 'projects/38554818207/secrets/ios-devicecheck-private-key/versions/1'),
  ('com.example.android.app', 'android', false, false, 60, 60, ARRAY ['GB','US'], false, NULL, NULL, NULL);

INSERT INTO exportconfig (filename_root, period_seconds, from_timestamp, thru_timestamp, region, bucket_name, signature_info_ids) VALUES
  ('exposureKeyExport-US', 300, '2020-05-01 07:00:00.000000', '2025-05-01 07:00:00.000000', 'US', 'apollo-public-bucket', '{1,2}');