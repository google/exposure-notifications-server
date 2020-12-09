# LatencyTooHigh

This alert is firing when the response latency is too high for one or
more services.

Check whether the latency is expected. Not all latency increase
have user impact and some services (e.g. export, cleanup) is expected to
have higher latency. If that's the case, consider adjust the alerting
threshold in [alerts.tf](../../../terraform/alerting/alerts.tf).

Reach out to internal chat if it cannot be explained by alerting
threshold.
