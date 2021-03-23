# Argo Service
![GitHub release (latest by date)](https://img.shields.io/github/v/release/keptn-contrib/argo-service)
[![Build Status](https://travis-ci.org/keptn-contrib/argo-service.svg?branch=master)](https://travis-ci.org/keptn-contrib/argo-service)
[![Go Report Card](https://goreportcard.com/badge/github.com/keptn-contrib/argo-service)](https://goreportcard.com/report/github.com/keptn-contrib/argo-service)

The Argo Service promotes [Argo Rollouts](https://argoproj.github.io/argo-rollouts/), which have been tested and evaluated with Keptn.


* This service listens on `sh.keptn.event.release.triggered` events and depending on the evaluation result *promotes* or *aborts* a rollout.
* This service derives the name of the rollout as well as the namespace from the service, stage, and project infos contained in the event.
More precisely, the rollout name is composed of `service`-`stage` and the namespace is composed of `project`-`stage`.

## Compatibility Matrix

| Keptn Version    | [Argo-service Service Image](https://hub.docker.com/r/keptncontrib/argo-service/tags) |
|:----------------:|:----------------------------------------:|
|   0.6.2    | keptncontrib/argo-service:0.1.0 |
|   0.7.0, 0.7.1    | keptncontrib/argo-service:0.1.1 |
|   0.7.2    | keptncontrib/argo-service:0.1.2 |
|   0.8.0 *)   | keptncontrib/argo-service:latest |

*) Not released yet.
 
## Deploy the Keptn-Argo service in your Kubernetes cluster

To deploy the current version of the *argo-service* in your Keptn Kubernetes cluster, use the file `deploy/service.yaml` from this repository and apply it:

```console
kubectl apply -f deploy/service.yaml -n keptn
```

## Delete the Keptn-Argo service in your Kubernetes cluster

To delete a deployed *argo-service*, use the file `deploy/service.yaml` from this repository and delete the Kubernetes resources:

```console
kubectl delete -f deploy/service.yaml -n keptn
```
