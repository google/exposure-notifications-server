
-- Use this as a template to inject configurations for your publish API endpoint.

INSERT INTO APIConfig
(app_package_name, enforce_apk_digest, cts_profile_match, basic_integrity, max_age_seconds, clock_skew_seconds, allowed_regions, all_regions, bypass_safetynet)
VALUES ('com.example.app', false, false, false, 120, 30, ARRAY ['GB','US'], false, true);
