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

ALTER TABLE ImportFilePublicKey
  ALTER export_import_id TYPE BIGINT,
  ALTER key_id TYPE TEXT,
  ALTER key_version TYPE TEXT,
  ALTER public_key TYPE TEXT;

CREATE INDEX importfilepublickey_from_timestamp ON ImportFilePublicKey(from_timestamp);
CREATE INDEX importfilepublickey_thru_timestamp ON ImportFilePublicKey(thru_timestamp);

DROP INDEX import_file_public_key_from;
DROP INDEX import_file_public_key_thru;

END;
