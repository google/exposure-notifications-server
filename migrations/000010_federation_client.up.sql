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

CREATE TABLE FederationAuthorization (
	oidc_issuer VARCHAR(1000) NOT NULL,
	oidc_subject VARCHAR(1000) NOT NULL,
	oidc_audience VARCHAR(1000),
	note VARCHAR(100),
	include_regions VARCHAR(5) [],
	exclude_regions VARCHAR(5) [],
	CONSTRAINT federation_authorization_pk PRIMARY KEY(oidc_issuer, oidc_subject)
);

END;
