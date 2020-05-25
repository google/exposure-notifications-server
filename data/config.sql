BEGIN; 
    INSERT INTO authorizedapp 
                (app_package_name, 
                 platform, 
                 allowed_regions, 
                 devicecheck_disabled, 
                 devicecheck_team_id, 
                 devicecheck_key_id, 
                 devicecheck_private_key_secret) 
    VALUES      ( 'uy.gub.app.covid19', 
                  'ios', 
                  '{"UY"}', 
                  true, 
                  'ABCD1234', 
                  'DEFG5678', 
                  NULL ); 

    INSERT INTO authorizedapp 
                (app_package_name, 
                 platform, 
                 allowed_regions, 
                 safetynet_disabled, 
                 safetynet_cts_profile_match, 
                 safetynet_basic_integrity, 
                 safetynet_past_seconds, 
                 safetynet_future_seconds) 
    VALUES      ( 'uy.gub.salud.plancovid19uy', 
                  'android', 
                  '{"UY"}', 
                  true, 
                  false, 
                  false, 
                  60, 
                  60 ); 

    INSERT INTO exportconfig 
    VALUES      (1, 
                 'mspuy', 
                 86400, 
                 '2020-01-01 00:00:00+00', 
                 '2021-01-01 00:00:00+00', 
                 'UY', 
                 'gx', 
                 '{1}'); 

END; 