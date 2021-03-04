# StackdriverExportFailed

OpenCensus failed to export metrics to Stackdriver.

This means all other alerts we have configured won't work as the alert depend
on the export metrics.

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

If the error can be correlated to a new release, reach out to the Slack
channel and ping the dev team for help.

If it's not correlated to a release, chances are the issue is on Google
Cloud Monitoring's end. Open a ticket with the Cloud Support team and
ping Slack channel to raise awareness.