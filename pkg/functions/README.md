# Cloud Functions

## Deploying

```
gcloud functions deploy SchedulerPubSubFunction --entry-point=SchedulerPubSubFunction  --runtime=go111 --trigger-topic=cron-topic
```

## Inspecting logs

```
gcloud functions logs read --filter="SchedulerPubSubFunction" --limit 50
```
