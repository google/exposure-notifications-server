# CloudSchedulerJobFailed

**Warning!** We are in the process of removing this alert in favor of `ForwardProgressFailed`. This alert and associated playbook will be removed in a future release.

Cloud Scheduler produced some ERROR level logs, indicating the job is
failing for some reason.

## Triage Steps

Go to Logs Explorer, use the following filter:

```
resource.type="cloud_scheduler_job"
severity=ERROR
```

See what the error message is. Depending on the error, you may also need
to check the corresponding Cloud Run service's log.
