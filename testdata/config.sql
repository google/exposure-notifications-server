
-- Use this as a template to inject configurations for your publish API endpoint.

INSERT INTO apiconfig (app_package_name, platform, cts_profile_match, basic_integrity, allowed_past_seconds, allowed_future_seconds, allowed_regions, all_regions) VALUES
  ('com.example.ios.app', 'ios', false, false, 60, 60, ARRAY ['GB','US'], false),
  ('com.example.android.app', 'android', false, false, 60, 60, ARRAY ['GB','US'], false);
