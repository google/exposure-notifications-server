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

CREATE TABLE Mirror(
    id SERIAL PRIMARY KEY,
    index_file VARCHAR(500) NOT NULL,
    export_root VARCHAR(500) NOT NULL,
    cloud_storage_bucket VARCHAR(200) NOT NULL,
    filename_root VARCHAR(500) NOT NULL
);

CREATE TABLE MirrorFile(
    mirror_id INT REFERENCES Mirror(id),
    filename VARCHAR(200) NOT NULL,
    PRIMARY KEY(mirror_id, filename)
);

END;
