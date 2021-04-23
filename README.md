# Argo Service
![GitHub release (latest by date)](https://img.shields.io/github/v/release/keptn-contrib/argo-service)
[![Build Status](https://travis-ci.org/keptn-contrib/argo-service.svg?branch=master)](https://travis-ci.org/keptn-contrib/argo-service)
[![Go Report Card](https://goreportcard.com/badge/github.com/keptn-contrib/argo-service)](https://goreportcard.com/report/github.com/keptn-contrib/argo-service)

The Argo Service currently supports:
* Canary and Blue/Green Deployments using [Argo Rollouts](https://argoproj.github.io/argo-rollouts/)

Future versions of this service may support additional integrations with other capabilities of the Argo suite of projects.

## Compatibility Matrix

| Keptn Version    | [Argo-service Service Image](https://hub.docker.com/r/keptncontrib/argo-service/tags) |
|:----------------:|:----------------------------------------:|
|   0.6.2    | keptncontrib/argo-service:0.1.0 |
|   0.7.0, 0.7.1    | keptncontrib/argo-service:0.1.1 |
|   0.7.2, 0.7.3    | keptncontrib/argo-service:0.1.2 |
|   0.8.0 - 0.8.2   | keptncontrib/argo-service:0.8.0 |

## Argo Rollout Support Explained

For a working example check out the tutorial [Keptn on k3s](https://github.com/keptn-sandbox/keptn-on-k3s/tree/release-0.8.0)

Assuming you have the following Keptn Project
* Project: `delivery-podtatohead`
* Service: `podtatohead`
* Stage: `prod`

**Helm Chart with Argo Rollout**
You should provide a Helm Chart that can be used with Keptn's Helm Service including Argo Rollout and Deployment definition like this:
```
kind: Rollout
metadata:
  name: {{ .Values.keptn.service }}-{{ .Values.keptn.stage }}
  ...
spec:
  replicas: {{ .Values.replicaCount }}
  strategy:
     canary:
       steps:
       - setWeight: 25
       - pause: {}
       - setWeight: 50
       - pause: {}
       - setWeight: 75
       - pause: {}
...
```

**Argo-Service will execute these argo rollout commands**
The `argo-service` will - depending on whether it promotes or aborts a rollout execute the following commands for you:
* promote: `kubectl argo rollouts promote rollout podtatohead-prod -n delivery-podtatohead-prod`
* abort: `kubectl argo rollouts abort rollout podtatohead-prod -n delivery-podtatohead-prod`

**The shipyard example for your Keptn project**
The shipyard for your Keptn project should contain
* deployment: which will have the helm service deploy your rollout definition
* test: to run some tests
* evaluation: to evaluate
* approve: for manual or automated approval

And then iterate through the promotion steps like this
* release: to promote the next rollout step
* test: more regular tests or canarywait
* evaluation: evaluate the current canary status
* approve: approve the evaluation result

Here is a sample shipyard file snippet:
```json
apiVersion: spec.keptn.sh/0.2.0
kind: Shipyard
metadata:
  name: "shipyard-delivery-rollout"
spec:
  stages:
  - name: prod           
    sequences:
    # Initial Deployment, Test and Evaluation
    - name: delivery        
      tasks:
      - name: deployment
        properties:
          deploymentstrategy: user_managed
      - name: test
        properties:
          teststrategy: performance
      - name: evaluation
      - name: approval
        properties:
          pass: manual
          warning: manual


    # Phase 1 of Rollout: Release, Test, Evaluate
    - name: canaryphase1
      triggeredOn:
        - event: prod.delivery.finished
          selector:
            match:
              result: "pass"
      tasks:
      - name: release
      - name: test
        properties:
          teststrategy: real-user
          canarywaitduration: 2m
      - name: evaluation
      - name: approval
        properties:
          pass: automatic
          warning: manual

    # Phase 2 of Rollout: Release, Test, Evaluate
    - name: canaryphase2
      triggeredOn:
        - event: prod.canaryphase1.finished
          selector:
            match:
              result: "pass"
      tasks:
      - name: release
      - name: test
        properties:
          teststrategy: real-user
          canarywaitduration: 2m
      - name: evaluation
      - name: approval
        properties:
          pass: automatic
          warning: automatic

    # Phase 3 (Final) of Rollout: Release, Test, Evaluate
    - name: canaryphase3
      triggeredOn:
        - event: prod.canaryphase2.finished
          selector:
            match:
              result: "pass"
      tasks:
      - name: release
      - name: test
        properties:
          teststrategy: real-user
          canarywaitduration: 2m
      - name: evaluation
      - name: approval
        properties:
          pass: automatic
          warning: automatic          
    - name: rollback
      triggeredOn:
      - event: prod.delivery.finished
        selector:
          match:
            result: "fail"
      - event: prod.canaryphase1.finished
        selector:
          match:
            result: "fail"
      - event: prod.canaryphase2.finished
        selector:
          match:
            result: "fail"
      - event: prod.canaryphase3.finished
        selector:
          match:
            result: "fail"            
      tasks:
      - name: rollback
```

**What really happens top to bottom?**
The argo-service registers for the following events:
* `sh.keptn.event.release.triggered` -> to handle promotion or abort of a rollout depending on evaluation result
* `sh.keptn.event.rollback.triggered` -> to handle abort of a rollout
* `sh.keptn.event.test.triggered` -> to handle the special canarywait teststrategy

Looking at the above shipyard - here is what happens
1 - `deployment`: The Helm-Service will deploy the Rollout definition. For the first deployment this will mean your current version will be fully rolled out to all replicas. For subsequent runs this will trigger the first rollout step phase 
2 - `test, evaluation, approval`: Keptn will trigger your test and then does the evaluation & approval
3 - `release`: `argo-service` will pick up the `release` task and will ONLY act if `deploymentstrategy` is `user_managed` or `duplicate` (blue/green) and the approval was passed! If so it will do a promote rollout
4: `test canarywait`: `argo-service` has its own wait handler implemented to mimick the pause between rollout steps. You don't have to use this as you can also just run any type of test. But - in a production deployment where you dont run additional tests you want to wait for a couple of minutes and then evaluate based on real user traffic. This is what this canarywait step does
5: `evaluate, approval`: same as above
6: repeat `release, test canarywait, evaluate, approval`: depending on how many steps you have in your canary rollout definition you should have the same number of iterations in the shipyard
7: `rollback`: if anything should fail during the rollout procedure the rollback sequence would cause `argo-service` to do an abort rollback bringing your system back to its previous version

## Open Items for implementation

Shall the arog-service also implement the deployment task that is currently handled by e.g: Helm Service?
This would allow the argo-service to do the
1: Initial `helm apply` for the first version and extract the deploymentURI e.g: from a well defined values.yaml property
2: Change the version and trigger a new rollout by calling `kubectl argo rollouts set image rollouts-demo rollouts-demo=argoproj/rollouts-demo:yellow` 

This would allow the Keptn user to do the initial deployment and then moving to the next version like this:
```console
keptn trigger delivery --project=myproject --service=myservice --stage=prod --image=myimage:1.0.0
keptn trigger delivery --project=myproject --service=myservice --stage=prod --image=myimage:2.0.0
```

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
