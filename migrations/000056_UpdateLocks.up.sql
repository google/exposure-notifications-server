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

CREATE INDEX lock_lock_id_expires ON Lock (lock_id, expires);

CREATE OR REPLACE FUNCTION AcquireLock(VARCHAR(100), INT) RETURNS TIMESTAMP AS $$
  DECLARE
    nowT TIMESTAMP;
    expiresT TIMESTAMP;
  BEGIN
    nowT := CURRENT_TIMESTAMP;
    expiresT := nowT + '1 SECOND'::interval * $2;

    -- Create the lock row. If it already exists, do nothing.
    INSERT INTO Lock (lock_id, expires) VALUES ($1, to_timestamp(0))
      ON CONFLICT (lock_id) DO NOTHING;

    -- Attempt to update the lock ttl if it's expired.
    UPDATE Lock SET expires = expiresT WHERE lock_id = $1 AND expires <= nowT;
    IF FOUND THEN
      -- The lock was acquired, return the new expiration time.
      RETURN expiresT;
    ELSE
      -- The current lock has not yet expired, return the sentinel value
      -- indicating the lock was not acquired.
      RETURN to_timestamp(0);
    END IF;
  END
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION ReleaseLock(VARCHAR(100), TIMESTAMP) RETURNS BOOLEAN AS $$
  BEGIN
    UPDATE Lock SET expires = to_timestamp(0)
      WHERE lock_id = $1 AND expires = $2;
    IF FOUND THEN
      -- Lock successfully expired.
      RETURN TRUE;
    ELSE
      -- Another process already expired the lock.
      RETURN FALSE;
    END IF;
  END
$$ LANGUAGE plpgsql;

END;
