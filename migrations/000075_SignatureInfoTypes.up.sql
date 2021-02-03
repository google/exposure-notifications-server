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

ALTER SEQUENCE signatureinfo_id_seq AS BIGINT;
ALTER TABLE SignatureInfo
  ALTER id TYPE BIGINT,
  ALTER signing_key TYPE TEXT,
  ALTER signing_key SET DEFAULT '',
  ALTER signing_key_version TYPE TEXT,
  ALTER signing_key_version SET DEFAULT '',
  ALTER signing_key_id TYPE TEXT,
  ALTER signing_key_id SET DEFAULT '';

CREATE INDEX signatureinfo_thru_timestamp ON SignatureInfo(thru_timestamp);

END;
