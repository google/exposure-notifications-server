# ForwardProgressFailed

This alert fires when background jobs have not made forward progress in an acceptable amount of time. The alert will include the name of the job that is failing to make forward progress. The jobs are invoked in the background.

- `backup` - Creates backups of the database.

- `cleanup-export` - Purges old export files.

- `cleanup-exposure` - Purges old exposures.

- `export-importer-import` - Imports export files from another server.

- `export-importer-schedule` - Downloads _index_ files from another server to be later imported by the `export-importer-import`

- `jwks` - Fetches and purges public keys from a public key discovery service.

- `key-rotation` - Rotates revision tokens and other keys. Note that the rotation service runs frequently, but only rotates keys when configured time intervals have passed. The system considers the `key-rotation` job to be successful even when no rotation is necessary.

- `mirror` - Mirrors exports from another server.

Each job runs on a different interval. Check your Terraform configuration to see how frequently a specific job runs.

## Triage Steps

When one of the jobs does not return success within a configured interval, this alert will fire. For most cases, this means the job has already failed 2+ times.

To begin triage, locate the logs for the corresponding service name using the Logs Explorer:

```text
resource.type="cloud_run_revision"
resource.labels.service_name="<service>"
```

For example, if the failing service was `cleanup-export`:

```text
resource.type="cloud_run_revision"
resource.labels.service_name="cleanup-export"
```

Check for errors in the logs.
