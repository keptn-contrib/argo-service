# Argo Service
![GitHub release (latest by date)](https://img.shields.io/github/v/release/keptn-contrib/argo-service)
[![Build Status](https://travis-ci.org/keptn-contrib/argo-service.svg?branch=master)](https://travis-ci.org/keptn-contrib/argo-service)
[![Go Report Card](https://goreportcard.com/badge/github.com/keptn-contrib/argo-service)](https://goreportcard.com/report/github.com/keptn-contrib/argo-service)

This service is used for promoting [Argo rollouts](https://argoproj.github.io/argo-rollouts/), which have been tested and evaluated with Keptn.


## Deploy the Keptn-Argo service in your Kubernetes cluster

To deploy the current version of the *argo-service* in your Keptn Kubernetes cluster, use the file `deploy/service.yaml` from this repository and apply it:

```console
kubectl apply -f deploy/service.yaml
```

## Delete the Keptn-Argo service in your Kubernetes cluster

To delete a deployed *argo-service*, use the file `deploy/service.yaml` from this repository and delete the Kubernetes resources:

```console
kubectl delete -f deploy/service.yaml
```
