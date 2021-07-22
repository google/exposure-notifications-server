-- Copyright 2021 the Exposure Notification Server authors
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

CREATE INDEX exposure_revised_at ON Exposure(revised_at);
CREATE INDEX exposure_revised_at_regions ON Exposure(revised_at, regions);
CREATE INDEX exposure_revised_at_local_provenance ON Exposure(revised_at, local_provenance);
CREATE INDEX exposure_revised_at_traveler ON Exposure(revised_at, traveler);

END;
