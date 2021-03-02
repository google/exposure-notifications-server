# HostDown

This is an uptime check on the custom domain. Failing these checks means
the clients cannot reach the verification server via the encv.org
hostnames.

The check happens every 60s, and this alert will fire if the server
failed the check for 5min.

## Triage steps

There are multiple components involved here so you need to thoroughly
check every one of them:

- Check the IP address assigned by Cloud Load Balancer: run `terraform
  output` and check the `lb_ip` from the result.
- Run `dig encv.org` (or the host that's down) and ensure the IP address
  matches the one you get above.
- Check https://status.cloud.google.com/ and see if any related services
  are having an outage right now.
- If none of the above steps yield useful insight, try follow
  http://cloud/load-balancing/docs/https/troubleshooting-ext-https-lbs
  and troubleshoot the issue assuming it's Cloud Load Balancer's
  problem.
- Check Cloud Run logs and see if there's any obvious error.
- If nothing obiously wrong, open a Cloud Support ticket.
