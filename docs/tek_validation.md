---
layout: default
---
# Recommended Temporary Exposure Key Validation

Here, we document the data and metadata validation approach taken by this server implementation.

These validations were developed in conjunction with the Android and iOS engineering teams
as we developed this server.

## Diagnosis verification

We recommend that a system is used to verify COVID diagnosis / test results.
For reference, see our [verification protocol design](design/verification_protocol.md).

## TEK validation

These are the validations that this server performs.

* number of proposed key uploads must be > 0
    * If violated, upload call fails.
* number of proposed key uploads must be <= configured max exposures (30 by default)
    * Reason: Most devices will have 15 TEKs to upload. If _future key upload_ is used, this could double to 30. Outside of testing, these numbers are practical.
    * If violated, upload call fails.
* TEK length must be exactly 16 bytes
    * If violated, that individual key is dropped and partial success could occur
* Report type, in valid range
    * Should be limited to valid report types for the server. For example this server only supports
      initial diagnosis of `CONFIRMED_TEST` or `CONFIRMED_CLINICAL_DIAGNOSIS`. On key revision,
      a clinical diagnosis can be changed to `CONFIRMED_TEST` or `REVOKED`
    * Initial (no revised) reports of `REVOKED` do not need to be saved.
* Roling start interval number, in valid range
    * the start interval should be >= 15 days ago (UTC day start) and <= now.
    * If violated, that individual key is dropped and partial success could occur
* Rolling period is >= 1 and <= 144
    * If violated, that individual key is dropped and partial success could occur
* Transmission risk >= 0 and <= 8
    * If violated, that individual key is dropped and partial success could occur
    * In our implementation, we allow TR of 0 to indicate that the server should fill in
      the appropriate value based on report type.
    * On device side matching, 0 is treated the same as 1
* Days since symptom onset, must be >= -14 and <= 14
    * If violated, the TEK is saved without a symptom onset and will undergo risk scoring without
    * This is a known bug in our implementation, but cannot be fixed due to launched clients. We recommend that if possible such TEKs be dropped from the upload.

### Embargo

* If a key received is still valid (roling start interal number is in the past, but rolling period indicates the key is still valid), that key should be embargoed until 2 hours after the end of the current UTC day.

## Revision validation

If a server is implementing `revised keys` as indicated in the export.proto format, it is important
that only the original uploader be allowed to change the key value.

Our implementation accomplishes this by returning an encrypted token on successful upload that must
be sent on future uploads.

The server should validate that only valid key transitions are allowed. There are only 2 valid
key transitions.

1. `CONFIRMED_CLINICAL_DIAGNOSIS` -> `CONFIRMED_TEST`
2. `CONFIRMED_CLINICAL_DIAGNOSIS` -> `REVOKED`
