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

ALTER SEQUENCE signatureinfo_id_seq AS INT;
ALTER TABLE SignatureInfo
  ALTER id TYPE INT,
  ALTER signing_key TYPE VARCHAR(500),
  ALTER signing_key_version TYPE VARCHAR(100),
  ALTER signing_key_id TYPE VARCHAR(50);

DROP INDEX IF EXISTS signatureinfo_thru_timestamp;

END;
