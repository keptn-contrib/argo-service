# Argo Service
![GitHub release (latest by date)](https://img.shields.io/github/v/release/keptn-contrib/argo-service)
[![Build Status](https://travis-ci.org/keptn-contrib/argo-service.svg?branch=master)](https://travis-ci.org/keptn-contrib/argo-service)
[![Go Report Card](https://goreportcard.com/badge/github.com/keptn-contrib/argo-service)](https://goreportcard.com/report/github.com/keptn-contrib/argo-service)

The Argo Service promotes [Argo rollouts](https://argoproj.github.io/argo-rollouts/), which have been tested and evaluated with Keptn.
Therefore, this service listens on `sh.keptn.events.evaluation-done` events and depending on the evaluation result
*promotes* or *aborts* a rollout.
This Argo Service derives the name of the rollout as well as the namespace from the service, stage, and project infos contained in the event.
More precisely, the rollout name is composed of `service`-`stage`
and the namespace is composed of `project`-`stage`.

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
