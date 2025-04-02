Run these commands

`/tmp/kube-burner init -c config.yml --uuid=demo`{{exec}}

List all created namespaces

```
kubectl get ns -l kube-burner-job=kube-burner-demo
kubectl get ns -l kube-burner-uuid=demo
```{{exec}}

List all deployments within a namespace

`kubectl get deploy -n kube-burner-demo-0`{{exec}}

List all deployments using label selector

`kubectl get deploy -A -l kube-burner-job=kube-burner-demo`{{exec}}

Manually destroy created stuff

`/tmp/kube-burner destroy --uuid=demo`{{exec}}

Set GC=true in your `config.yml` and re-run:

```yaml
global:
  gc: true                      # Garbage collection set to true
jobs:
  - name: kube-burner-demo       # job name
    namespace: kube-burner-demo  # namespace prefix
    jobIterations: 1            # number of job iterations
    qps: 5                       # API request rate
    burst: 5
    namespacedIterations: true   # Create a new namespace per job iteration
    waitWhenFinished: true       # Wait for pods to be running before finishing the job
    preLoadImages: true          # Preload pod images
    preLoadPeriod: 10s
    cleanup: true                # Cleanup resources from previous runs
    namespaceLabels:             # Add additional labels to the created namespaces
      pod-security.kubernetes.io/enforce: privileged
      pod-security.kubernetes.io/audit: privileged
      pod-security.kubernetes.io/warn: privileged
    objects:
      - objectTemplate: templates/deploy.yml
        replicas: 10
        inputVars:
          podReplicas: 1 
          image: quay.io/cloud-bulldozer/nginx:latest
```

`/tmp/kube-burner init -c config.yml`{{exec}}

Demonstrate that all namespaces were garbage collected at the end of the run

`kubectl get ns | grep kube-burner-demo`{{exec}}