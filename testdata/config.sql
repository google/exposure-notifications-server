
-- Use this as a template to inject configurations for your publish API endpoint.

INSERT INTO apiconfig (app_package_name, platform, cts_profile_match, basic_integrity, allowed_past_seconds, allowed_future_seconds, allowed_regions, all_regions, ios_devicecheck_team_id_secret, ios_devicecheck_key_id_secret, ios_devicecheck_private_key_secret) VALUES
  ('com.example.ios.app', 'ios', false, false, 60, 60, ARRAY ['UY'], false, 'projects/38554818207/secrets/ios-devicecheck-team-id/versions/1', 'projects/38554818207/secrets/ios-devicecheck-key-id/versions/1', 'projects/38554818207/secrets/ios-devicecheck-private-key/versions/1'),
  ('com.example.android.app', 'android', false, false, 60, 60, ARRAY ['UY'], false, NULL, NULL, NULL);
