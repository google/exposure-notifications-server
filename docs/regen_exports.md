# Regenerating exports

Should you need to regenerate exports.

1) Pause the scheduler for the `/do-work` invoker of the export job.
2) Delete the exportfiles for the files you want to regnerate.
3) Mark the batches as `'OPEN'` for the batches you want to regenerate.
4) Increment the value of the `REPROCESS_COUNT` environment variable on the export service.
5) Restart all instances of the export service.
6) Unpause the `/do-work` invoker.