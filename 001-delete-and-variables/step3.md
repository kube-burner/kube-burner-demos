Explain how functions from [sprig](http://masterminds.github.io/sprig/) can be called, [working example](templates/secret.yml)

`/tmp/kube-burner init -c config.yml`{{exec}}

Demonstrate that only deployments have been deleted in the namespace
`kubectl get deploy,secret -n create-stuff`{{exec}}