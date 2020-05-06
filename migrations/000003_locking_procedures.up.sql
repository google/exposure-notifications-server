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

CREATE OR REPLACE FUNCTION AcquireLock(VARCHAR(100), INT) RETURNS TIMESTAMP AS $$
	DECLARE
		nowT TIMESTAMP;
		expiresT TIMESTAMP;
	BEGIN
		nowT := CURRENT_TIMESTAMP;
		expiresT := nowT + '1 SECOND'::interval * $2;

		IF EXISTS (SELECT lock_id FROM Lock WHERE lock_id = $1 AND expires > nowT) THEN
			RETURN to_timestamp(0); -- Special value indicating no lock acquired.
		END IF;

		IF EXISTS (SELECT lock_id FROM Lock WHERE lock_id = $1) THEN
			UPDATE Lock SET expires = expiresT WHERE lock_id = $1;
			RETURN expiresT;
		END IF;

		INSERT INTO Lock (lock_id, expires) VALUES ($1, expiresT);
		RETURN expiresT;
	END
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION ReleaseLock(VARCHAR(100), TIMESTAMP) RETURNS BOOLEAN AS $$
	BEGIN
		IF NOT EXISTS (SELECT lock_id FROM Lock WHERE lock_id = $1 AND expires = $2) THEN
			RETURN FALSE; -- Another process acquired an expired lock
		END IF;

		DELETE FROM Lock WHERE lock_id = $1;
		RETURN TRUE;
	END
$$ LANGUAGE plpgsql;

END;
