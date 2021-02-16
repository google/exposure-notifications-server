# HumanDecryptedValue

This alert fires when a non-service account decrypts a value using Cloud KMS. It
only fires once in the period, even if multiple decryption events occur.

## Triage Steps

Go to Logs Explorer, use the following filter:

```
protoPayload.@type="type.googleapis.com/google.cloud.audit.AuditLog"
protoPayload.serviceName="cloudkms.googleapis.com"
protoPayload.methodName="Decrypt"
protoPayload.authenticationInfo.principalEmail!~"gserviceaccount.com$"
```

Expand the JSON fields to identify who performed a decryption operation.
