## How delete Jobs works

Explain how a delete job works and how labelSelector, apiVersion and kind need to be configured

Show how variables work within kube-burner templates

```yaml
inputVars:
  foo: bar
```

Explain how functions from [sprig](http://masterminds.github.io/sprig/) can be called, [working example](templates/secret.yml)

```shell
kube-burner init -c config.yml
# Demonstrate that only the deployments have been deleted in the namespace
kubectl get deploy,secret -n create-stuff
```
