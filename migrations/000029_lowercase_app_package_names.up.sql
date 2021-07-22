-- Copyright 2020 the Exposure Notification Server authors
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

UPDATE AuthorizedApp SET app_package_name = LOWER(app_package_name)
  WHERE app_package_name IS NOT NULL;

UPDATE Exposure SET app_package_name = LOWER(app_package_name)
  WHERE app_package_name IS NOT NULL;

UPDATE SignatureInfo SET app_package_name = LOWER(app_package_name)
  WHERE app_package_name IS NOT NULL;
UPDATE SignatureInfo SET bundle_id = LOWER(bundle_id)
  WHERE bundle_id IS NOT NULL;

END;
