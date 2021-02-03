-- Copyright 2021 Google LLC
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
  ALTER app_package_name TYPE VARCHAR(1000),
  ALTER allowed_regions TYPE VARCHAR(5)[],
  ALTER allowed_health_authority_ids TYPE INT[];

DROP INDEX IF EXISTS authorizedapp_allowed_regions;
DROP INDEX IF EXISTS authorizedapp_allowed_health_authority_ids;

END;
