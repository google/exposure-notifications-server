# Google Exposure Notifications Server - Deployment Options Requirements
Here we provide some distinct deployment options for deploying the exposure notification server components.

Created: 2020-04-22

Updated: 2020-05-09



## Objective

This document describes potential strategies for building and hosting the server. For more details on each component, please view the [Server Functional Requirements](server_functional_requirements.md).

We have outlined a number of options, ranging from an entirely self-hosted service to a fully managed Google Cloud deployment. The BLE Proximity Notifications Server architecture can be deployed in the following environments:


*   Fully self-hosted or on-premises
*   Fully managed Google Cloud
*   A combination of self-hosted and fully managed

This document should help you compare and explore trade-offs when making hosting decisions.


## Components to Host

As part of the reference architecture, there are multiple discrete components that can be hosted separately.

The server components can be separated as follows:


*   Exposure API Server
*   Database
*   Data Cleanup Job
*   Batch Pushes to Storage/CDN
*   Infected key server (Cloud CDN in the above diagram)
*   Secret Management (Signing Secrets, Federation Secrets, etc)
*   Job Scheduling
*   (OPTIONAL) Federated Ingestion (for integrating with iOS/Apple devices)
*   (OPTIONAL) Federated API Server



![alt_text](images/compute_data_in.png "image_tooltip")


![alt_text](images/compute_data_out.png "image_tooltip")



## Components not covered as part of this document


### Federated learning and federated analytics

This document covers consumption and storage of anonymised temporary tracing keys.



## Overview of recommended data deployment approaches


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
   <td><strong>Database</strong>
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
   <td><strong>Key Batch CDN</strong>
   </td>
   <td>Daily batches of database for client use.
   </td>
   <td><a href="https://cloud.google.com/cdn">Google Cloud CDN</a>
   </td>
   <td>Third Party CDN, Redis + Server, or allow direct access to storage
   </td>
   <td>Third Party CDN, Redis + Server, or allow direct access to storage
   </td>
  </tr>
  <tr>
   <td><strong>Key Batch Storage</strong>
   </td>
   <td>Daily batches of database for client use.
   </td>
   <td><a href="https://cloud.google.com/storage">Google Cloud Storage</a>
   </td>
   <td>Open Source Blobstore hosted with on-prem Kubernetes
   </td>
   <td>Kubernetes hosted Open Source Blobstore (ie. min.io, rook). Could also use Redis and reconstruct batches
   </td>
  </tr>
  <tr>
   <td><strong>Secret Management</strong>
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
</table>




## Overview of recommended compute approaches


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
   <td><strong>Exposure Ingestion API Server</strong>
   </td>
   <td>Ingestion of exposure keys from android devices. 
<p>
Could be extended for cancellation.
   </td>
   <td><a href="https://cloud.google.com/run">Google Cloud Run</a>
   </td>
   <td>On-prem Kubernetes with Cloud Run for <a href="https://cloud.google.com/anthos">Anthos GKE on-prem</a>
   </td>
   <td>Kubernetes with Knative Serving
   </td>
  </tr>
  <tr>
   <td><strong>Exposure Reporting Server</strong>
   </td>
   <td>Serves infected keys to users
   </td>
   <td><a href="https://cloud.google.com/storage">Google Cloud Storage</a> + <a href="https://cloud.google.com/cdn">Google Cloud CDN</a>
   </td>
   <td>On-prem Kubernetes with <a href="https://cloud.google.com/anthos">Anthos GKE on-prem</a>, Or Google Cloud CDN
   </td>
   <td>Kubernetes with Knative Serving + Redis
   </td>
  </tr>
  <tr>
   <td><strong>Exposure Data Deletion</strong>
   </td>
   <td>Removal of data older than a configurable time, for instance 14D
   </td>
   <td><a href="https://cloud.google.com/run">Google Cloud Run</a>
   </td>
   <td>On-prem Kubernetes with <a href="https://cloud.google.com/anthos">Anthos GKE on-prem</a>
   </td>
   <td>Kubernetes, either job or Knative Service 
   </td>
  </tr>
  <tr>
   <td><strong>Batch Pushes to Storage/CDN</strong>
   </td>
   <td>Periodic DB queries to batch data for client consumption \
 \
Signs payloads for verification on device.
   </td>
   <td><a href="https://cloud.google.com/run">Google Cloud Run</a>
   </td>
   <td>On-prem Kubernetes with <a href="https://cloud.google.com/anthos">Anthos GKE on-prem</a>
   </td>
   <td>Kubernetes, either job or Knative Service
   </td>
  </tr>
  <tr>
   <td><strong>Job Scheduling</strong>
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
   <td><strong>(OPTIONAL) Federated Ingestion</strong>
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
   <td><strong>(OPTIONAL) Federated Sharing</strong>
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



## 


## 1. GKE On-Prem

Data and Compute Hosted on Premises


![alt_text](images/on_prem.png "image_tooltip")



### Considerations of operating solution

This solution allows for complete control of all components in any location where a datacenter exists. The cost comes in the operation of that hosting.

While not outlined in the diagram or table, for this solution it is suggested that you build audit logging/access logging to data and endpoints (such functionality is built into the other two options).


## 2. Self-hosted, Google-managed

Data hosted on TIM, Compute hosted on Google Cloud

There is flexibility for how you combine on-premise and cloud-only solutions. For this example, let’s assume all serverless functions would be hosted on Google Cloud, with databases hosted on premise.

Note: while not pictured, you can also consider a similar solution, where the processing happens on an [Anthos](https://cloud.google.com/anthos) cluster on-premise and the data is stored in Google Cloud as a fully managed service. 


![alt_text](images/hybrid_in.png "image_tooltip")


![alt_text](images/hybrid_out.png "image_tooltip")


**3. Fully Hosted Google Cloud (this is included here as a reference)**

Data and Compute hosted on Google Cloud


![alt_text](images/google_cloud_run.png "image_tooltip")



### Considerations of operating solution

By using fully hosted components much of the service’s operation can be delegated to Google Cloud and provide auditing of access to data. For example, Hosted Postgres delegates upgrades and patching of the database to Google Cloud. 

This solution requires hosting within a [Google Cloud location](https://cloud.google.com/about/locations) which may not exist in a location that permits use for all aspects of the design. For instance, within the EU, components could host compute resources in Belgium, Netherlands, or Finland. Data could be hosted in London, Belgium, Netherlands, Zurich, Frankfurt, or Finland.


## Change Log

* 22 Apr 2020 - Initial document - crwilcox
* 27 Apr 2020 - Updated diagrams for scenarios to be clearer - crwilcox
* 6 May 2020 - Changes to make document more general - crwilcox

