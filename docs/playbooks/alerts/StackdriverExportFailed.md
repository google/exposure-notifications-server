# StackdriverExportFailed

OpenCensus failed to export metrics to Stackdriver. This means that alerts
that depends on the exports metrics may not work.

NOTE: metric export may spontanously fail. If the failure rate is low it's
likely the threshold is too sensitive.

## Triage steps

You can use the following query to get the rate of errors:

```
fetch cloud_run_revision
| metric 'logging.googleapis.com/user/stackdriver_export_error_count'
| align rate(1m)
| every 1m
| group_by [resource.revision_name, resource.service_name],
    [row_count: row_count()]
```

If the error can be correlated to a new release, please open a new Github 
issue and provide the log information you have.

If it's not correlated to a release, chances are the issue is on Google
Cloud Monitoring's end. Consider checking [Google Cloud Status Dashboard](https://status.cloud.google.com/)
or opening a Support ticket with your Cloud provider.