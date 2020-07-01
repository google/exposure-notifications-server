---
layout: default
---
# Estimating costs of hosting the Exposure Notification Key Server

    NOTE: This is for informational purposes only. This does
    not account for all costs of operating. This document
    should be used to assist in understanding how to calculate
    the cost of deploying this service, not to make budgetary
    decisions.

This page outlines how you might estimate the cost of deploying the reference
server for your application. Cost estimates assume the use of Google Cloud,
deployment and use within the US, and uses the
[published pricing rates](https://cloud.google.com/pricing) to arrivea at the
estimation. This could be repeated for other regions as well as for other
cloud providers. Please note that for much of this simplifications have been
made. Due to a majority of the expected costs to be incurred due to storage
serving.

# Calculating the overall cost
There are a number of variables that need to be figured out to determine cost.

Let's assume that we are calculating the cost of operating for an area with:

* A population of 10 Million (10,000,000) people.  
* An exposure window of 14 days
* 50% adoption of an exposure notifications system
* 1,250 new cases daily (approximately .01% new cases per day)

From which we can derive:
* Each new set of keys is 448 bytes (14 days * 32 bytes)
* Each daily batch is 1,250 * 448 bytes = .56 MB
* That file is downloaded 5 Million times per day, for (2.8 TB/day downloaded),
  84 TB monthly.

# Cloud Components and Pricing
Due to pricing varying by region, and much of this being scalable, we avoid
speaking about exact pricing for most of it, instead trying to quantify the
magnitude.

To have an idea of the deployment size, this document assumes the defaults
configured for Terraform in
[vars.tf](https://github.com/google/exposure-notifications-server/blob/main/terraform/vars.tf).

The largest cost is likely to be in Network Egress. In this calculation we
assume that [Google Cloud CDN](https://cloud.google.com/cdn) is not used but
instead a [Google Cloud Storage](https://cloud.google.com/storage) bucket is
read directly. Please note that using Google Cloud CDN will likely result in
lower cost.

## Cloud Run Costs
https://cloud.google.com/run/pricing

There are multiple containers needed for a complete deployment:

* Export Cleanup
* Exposure Cleanup
* Export
* Exposure
* Federation In
* Federation Out

Most of the services run periodically, not constantly. For this reason it is
likely no higher than 6 containers at this scale. Likely, estimating as if 3-4
of these containers would get a reasonable upper limit. The resource limits of
a container are specified in the
[terraform resource limits](https://github.com/google/exposure-notifications-server/blob/main/terraform/service_federationin.tf#L63)
as 2 CPU and 1G memory per container.

**Projected Monthly Cost: $500 - $750**

## SQL Database Costs
https://cloud.google.com/sql/pricing

The default size for SQL Database is configured to:
* 8 vCPU
* 30720 MB Memory
* 256 GB SSD

The cost of this component can vary greatly on the scale is configured to.
Depending on the needed scale of your deployment this could require a different
configuration.

For instance, while the default is `db-custom-8-30720`, the documentation
mentions that a `db-custom-1-3840` instance is likely sufficeint for local
development work. This instance should cost around an eighth of the amount.

**Project Monthly Cost: $1000 - $1250**

## Storage Costs
https://cloud.google.com/storage/pricing

### Data Storage Cost
The last 14 days of batches are stored with older batches being deleted. This
is a small amount of data storage. 14 days of batches should be around 8 MB.

**Projected Monthly Cost: $0 - $10**

### Network Usage Cost
Each user will download one of the daily batch files. This is a highly variable
cost based on adoption. With sufficient users this could easily be in the tens
of thousands of dollars. Also, network egress varies depending on the region.
Please see the [pricing](https://cloud.google.com/storage/pricing#network-egress)
for network usage to determine a more accurate estimate for your use case.

Copied from above:
* Each new set of keys is 448 bytes (14 days * 32 bytes)
* Each daily batch is 1,250 * 448 bytes = .56 MB
* That file is downloaded 5 Million times per day, for (2.8 TB/day downloaded),
  84 TB monthly.

**Projected Monthly Cost: $10,000+**

    NOTE: Using the Google Cloud CDN is likely less expensive.
    Serving costs when using Google Cloud Storage directly are
    higher, often by more than 30%, than Google Cloud CDN.

### Operations Cost
There are two tiers of operations.

#### Class A
$0.05 per 10,000 operations

Relevant operations from this include `INSERT`, `UPDATE`, and `LIST`. These
will occur as part of batching of exposure keys. These operations will likely
remain under 10,000 for storage as creation of files is batched.

**Projected Monthly Cost: $0 - $20**

#### Class B
$0.004 per 10,000 operations

Relevant operations from this include `GET` requests for key batches.

For each user, there will be a minimum of one `GET` operation per day.

**Projected Monthly Cost: $50 - $100**
