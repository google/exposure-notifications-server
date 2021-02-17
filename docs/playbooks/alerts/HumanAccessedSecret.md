# HumanAccessedSecret

This alert fires when a non-service account accessed a secret. It only fires
once in the period, even if multiple secrets are accessed.

## Triage Steps

Go to Logs Explorer, use the following filter:

```text
protoPayload.@type="type.googleapis.com/google.cloud.audit.AuditLog"
protoPayload.serviceName="secretmanager.googleapis.com"
protoPayload.methodName=~"AccessSecretVersion$"
protoPayload.authenticationInfo.principalEmail!~"gserviceaccount.com$"
```

Expand the JSON fields to identify which secret(s) were accessed and which
human(s) performed the access.
