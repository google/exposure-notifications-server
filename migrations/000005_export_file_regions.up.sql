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

ALTER TABLE ExportConfig DROP COLUMN include_regions;
ALTER TABLE ExportConfig DROP COLUMN exclude_regions;
ALTER TABLE ExportBatch DROP COLUMN include_regions;
ALTER TABLE ExportBatch DROP COLUMN exclude_regions;

ALTER TABLE ExportConfig ADD COLUMN region VARCHAR(5);
ALTER TABLE ExportBatch ADD COLUMN region VARCHAR(5);

END;
