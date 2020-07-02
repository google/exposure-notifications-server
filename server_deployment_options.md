---
layout: default
---
# Google Exposure Notification Key Server

## Server deployment options

This document describes possible strategies for building and hosting the
Exposure Notification Key Server components. You should use this information
explore and compare trade-offs when making hosting decisions.

The Exposure Notification Key Server can be deployed in the following
environments:

* Fully self-hosted or on-premises
* Fully managed Google Cloud
* A combination of self-hosted and fully managed

For more details on each
component, see the [Server Functional Requirements](server_functional_requirements.md).

## Server architecture

The Exposure Notification Key Server has multiple components which can be
categorized as compute and data. To understand deployment scenarios, you should
look at the architecture of the server and data flow between servers and
devices.

![Exposure Notification Key Server data ingress flow](images/data-ingress.svg "Exposure Notification Key Server data ingres flow")

![Exposure Notification Key Server data egress flow](images/data-retrieval.svg "Exposure Notification Key Server data egress flow")

The Exposure Notification Key Server compute components have been designed to be stateless,
scalable, and rely on data stored in a shared databased. This makes the compute
components suited to deployment on
[serverless compute platforms](https://en.wikipedia.org/wiki/Serverless_computing).

Serverless platforms can scale down to zero during times of zero usage, which is
likely if the Exposure Notification Key Server deployment covers a single or small
number of countries. During times of high demand, serverless platforms can
scale to meet demand.

## Server components

<table>
  <tr>
   <td>
   </td>
   <td><strong>Purpose</strong>
   </td>
   <td><strong>Fully Hosted Google Cloud</strong>
   </td>
   <td><strong>Self-hosted, Google-managed</strong>
   </td>
   <td><strong>Self Hosted Kubernetes</strong>
   </td>
  </tr>
  <tr>
    <td colspan="5">
    <strong>Data components</strong>
    </td>
  </tr>
  <tr>
   <td><strong>Exposure key database</strong>
   </td>
   <td>Stores anonymized exposure keys from devices identified as exposed
   </td>
   <td><a href="https://cloud.google.com/sql/">Google Cloud SQL (PostgreSQL)</a>
   </td>
   <td>PostgreSQL hosted with on-prem Kubernetes
   </td>
   <td>PostgreSQL on Kubernetes
   </td>
  </tr>
  <tr>
   <td><strong>Exposure key batches storage</strong>
   </td>
   <td>Storing batches of exposure keys that will be sent to devices.
   </td>
   <td><a href="https://cloud.google.com/storage/">Google Cloud Storage</a>
   </td>
   <td>Open Source Blobstore hosted with on-prem Kubernetes
   </td>
   <td>Kubernetes hosted Open Source Blobstore (ie. min.io, rook). Could also use Redis and reconstruct batches
   </td>
  </tr>
  <tr>
   <td><strong>Certificate and key storage</strong>
   </td>
   <td>Secure Storage for secrets such as signing, private keys, etc.
   </td>
   <td><a href="https://cloud.google.com/secret-manager">Secret Manager</a>
   </td>
   <td><a href="https://cloud.google.com/anthos">Anthos GKE on Prem</a> + KMS
   </td>
   <td>HashiCorp Vault
   </td>
  </tr>
  <tr>
    <td colspan="5">
    <strong>Compute components</strong>
    </td>
  </tr>
  <tr>
   <td><strong>Exposure key ingestion server</strong>
   </td>
   <td>Ingestion of exposure keys from client devices.
   </td>
   <td><a href="https://cloud.google.com/run/">Google Cloud Run</a>
   </td>
   <td>On-prem Kubernetes with Cloud Run for <a href="https://cloud.google.com/anthos">Anthos GKE on-prem</a>
   </td>
   <td>Kubernetes with Knative Serving
   </td>
  </tr>
  <tr>
   <td><strong>Exposure Reporting Server</strong>
   </td>
   <td>Serves anonymous keys of exposed users
   </td>
   <td><a href="https://cloud.google.com/storage/">Google Cloud Storage</a> + <a href="https://cloud.google.com/cdn">Google Cloud CDN</a>
   </td>
   <td>On-prem Kubernetes with <a href="https://cloud.google.com/anthos">Anthos GKE on-prem</a>, Or Google Cloud CDN
   </td>
   <td>Kubernetes with Knative Serving + Redis
   </td>
  </tr>
  <tr>
   <td><strong>Data deletion</strong>
   </td>
   <td>Deletion of data that is older than a configured time limit.
   </td>
   <td><a href="https://cloud.google.com/run/">Google Cloud Run</a>
   </td>
   <td>On-prem Kubernetes with <a href="https://cloud.google.com/anthos">Anthos GKE on-prem</a>
   </td>
   <td>Kubernetes, either a job or Knative Service
   </td>
  </tr>
  <tr>
   <td><strong>Batch exposure keys</strong>
   </td>
   <td>Periodic DB queries to batch data for client consumption.
       Signing payloads for verification on device.
   </td>
   <td><a href="https://cloud.google.com/run/">Google Cloud Run</a>
   </td>
   <td>On-prem Kubernetes with <a href="https://cloud.google.com/anthos">Anthos GKE on-prem</a>
   </td>
   <td>Kubernetes, either job or Knative Service
   </td>
  </tr>
  <tr>
   <td><strong>Periodic data batching and deletion</strong>
   </td>
   <td>Used to control running of periodic jobs (deletion, batching)
   </td>
   <td><a href="https://cloud.google.com/scheduler">Google Cloud Scheduler</a>
   </td>
   <td>Kubernetes Cronjobs
   </td>
   <td>Kubernetes Cronjobs
   </td>
  </tr>
  <tr>
   <td><strong>Content delivery network</strong>
   </td>
   <td>Distribution of keys to client devices
   </td>
   <td><a href="https://cloud.google.com/cdn/">Google Cloud CDN</a>
   </td>
   <td>Third Party CDN, Redis + Server, or allow direct access to storage
   </td>
   <td>Third Party CDN, Redis + Server, or allow direct access to storage
   </td>
  </tr>
  <tr>
    <td colspan="5">
    <strong>Optional components</strong>
    </td>
  </tr>
  <tr>
   <td><strong>Federated ingestion</strong>
   </td>
   <td>Ingestion of keys from other parties.
   </td>
   <td><a href="https://cloud.google.com/run">Google Cloud Run</a>
   </td>
   <td>On-prem Kubernetes with <a href="https://cloud.google.com/anthos">Anthos GKE on-prem</a>
   </td>
   <td>Kubernetes with Knative Serving
   </td>
  </tr>
  <tr>
   <td><strong>Federated acesss</strong>
   </td>
   <td>Allows other parties/countries to retrieve data
   </td>
   <td><a href="https://cloud.google.com/run">Google Cloud Run</a>
   </td>
   <td>On-prem Kubernetes with <a href="https://cloud.google.com/anthos">Anthos GKE on-prem</a>
   </td>
   <td>Kubernetes with Knative Serving
   </td>
  </tr>
</table>

## Hosting infrastructure options

### Data and compute hosted on premises

You can host all components of the Exposure Notification Key Server on-premises.

![A diagram of the Exposure Notification Key Server deployed on-premises](images/on_prem.png "Exposure Notification Key Server on-premises deployment")

Deploying compute and data components on-premises allows you to have complete
control of all components and deploy them in any location by using an
on-premises Google Kubernetes Engine cluster. However, an on-premises
deployment will require you to configure and maintain the underlying
infrastructure, and ensure it is able to meet usage demands.

When the Exposure Notification Key Server is deployed on-premises, we recommend you
deploy audit and access logging to the data and API endpoints. This is
automatically available in the fully managed, and hybrid deployment scenarios.

### Storing data on-premises and using Google-managed compute

The Exposure Notification Key Server supports either compute or data components to
be hosted on-premises or on Google Cloud.

![A diagram of data ingress with the Exposure Notification Key Server that has compute components on Google Cloud and data on-premises](images/hybrid_in.png "image_tooltip")

![A diagram of data egress with the Exposure Notification Key Server that has compute components on Google Cloud and data on-premises](images/hybrid_out.png "image_tooltip")

This example deployment has compute components running on Google Cloud
Serverless products, with databases hosted on-premises. Alternatively, you
could use an [Anthos](https://cloud.google.com/anthos/) cluster to host
compute components on premises, and have the data components hosted on Google
Cloud as a fully managed service.

### Fully hosted on Google Cloud

This example deployment hosts all components of the system on Google Cloud.

![A diagram of the Exposure Notification Key Server deployed on Google Cloud](images/google_cloud_run.png "Exposure Notification Key Server deployed on Google Cloud")

By using fully hosted components most of the service’s operation can be
delegated to Google Cloud, which will provide audit and access logging of the
data. For example, Cloud SQL will manage the infrastructure for a hosted
PostgreSQL database.

This solution requires hosting within a
[Google Cloud location](https://cloud.google.com/about/locations) which may not
exist in a location that permits use for all parts of the Exposure
Notification Server architecture.