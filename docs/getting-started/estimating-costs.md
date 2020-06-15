---
layout: default
---
# Estimating costs of hosting the Exposure Notifications Server


NOTE: This is for informational purposes only. This doesn't account for all
costs of operating. It gives an approximate sizing of the cost. This document
should be used to assist in understanding how to calculate the cost of
deploying this service.

This page explains how you might estimate the cost to deploy servers within the
Exposure Notification Reference implementation. Cost estimates assume the use
of Google Cloud and use
[published pricing rates](https://cloud.google.com/pricing), but similar
calculations could be done for other cloud providers as well as self-hosted
machines.

Much of this is simplified. It doesn't
fully account for free-tier exceptions, or tiered pricing. This is meant
to assist in having an approximate idea of cost to operate. This also assumes
the defaults at the time of authoring in [vars.tf](https://github.com/google/exposure-notifications-server/blob/master/terraform/vars.tf).

These estimates also don't include CDN costs. For simplicity they assume
direct access to Cloud Storage.

# Calculating the overall cost
There are a number of variables that need to be figured out to determine cost.
That said, the vast majority of that cost is going to be from the network
egress. The next largest cost is likely for hosting of containers.

Let's assume that we are calculating the cost of operating for an area with:

* A population of 10 Million (10,000,000) people.  
* An exposure window of 14 days
* 50% adoption of an exposure notifications system
* 1,250 new cases daily (approxmately .01% new cases per day)

From which we can derive:
* Each new set of keys is 280 bytes (14 days * 20 bytes)
* Each daily batch is 1,250 * 280 bytes = .35 MB
* That file is downloaded 5 Million times per day (1.75 GB/day download) for
  a total monthly egress of 52.5 GB.

# Cloud Components and Pricing

## Cloud Run Costs
https://cloud.google.com/run/pricing

There are multiple containers needed for a complete deployment: 

* Export Cleanup
* Exposure Cleanup
* Export
* Exposure
* Federation In
* Federation Out

Let's assume we run ~4 containers constantly as much of the operation is batch
and won't account for constant use. Also, we can reflect on the default
[terraform resource limits](https://github.com/google/exposure-notifications-server/blob/master/terraform/service_federationin.tf#L63)
of 2 CPU and 1G memory per container.

**Projected Monthly Cost: $500 - 1000**

## SQL Database Costs
https://cloud.google.com/sql/pricing

The default size for SQL Database is configured to:
* 8 vCPU
* 30720 MB Memory
* 256 GB SSD

The cost of this component can vary greatly on the scale is configured to.
Depending on the needed scale of your deployment this could require a different
configuration.

**Project Monthly Cost: $1000 - $1250**

## Storage Costs
https://cloud.google.com/storage/pricing

### Data Storage Cost
The last 14 days of batches are stored with older batches being deleted. This
is a small amount of data storage.

**Projected Monthly Cost: $0 - $20.00**

### Network Usage Cost
Each day every user will need to download a batch. This means that roughly
5M * 3.5MB
$0.12 per GB (egress)

**Projected Monthly Cost: ($0.12 / GB) * (1.75GB/day * 30 days) = $6.30**

### Operations Cost
There are two tiers of operations with different billing.

#### Class A
$0.05 per 10,000 operations

Relevant operations from this include `INSERT`, `UPDATE`, and `LIST`. These
will occur as part of batching of exposure keys. These operations will likely
remain under 10,000 for storage as creation of files is batched.

**Projected Monthly Cost: $0 - $20.00**

#### Class B
$0.004 per 10,000 operations

Relevant operations from this include `GET` requests for key batches.

For each user, there will be a minimum of one `GET` operation per day.

**Projected Monthly Cost: $50 - $100**

