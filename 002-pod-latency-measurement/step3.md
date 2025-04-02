Running kube-burner with podLatency measurement

`/tmp/kube-burner init -c config.yml`{{exec}}


Check log lines

```
time="2024-01-18 01:07:01" level=info msg="Evaluating latency thresholds" file="metrics.go:48"           
time="2024-01-18 01:07:01" level=info msg="kube-burner-demo: Ready 50th: 2000 99th: 3000 max: 3000 avg: 1930" file="pod_latency.go:235"     
time="2024-01-18 01:07:01" level=info msg="kube-burner-demo: PodScheduled 50th: 0 99th: 0 max: 0 avg: 0" file="pod_latency.go:235"          
time="2024-01-18 01:07:01" level=info msg="kube-burner-demo: ContainersReady 50th: 2000 99th: 3000 max: 3000 avg: 1930" file="pod_latency.go:235"         
time="2024-01-18 01:07:01" level=info msg="kube-burner-demo: Initialized 50th: 0 99th: 0 max: 0 avg: 0" file="pod_latency.go:235"           
time="2024-01-18 01:07:01" level=error msg="podLatency: P99 Ready latency (3.00s) higher than configured threshold: 2s" file="job.go:189"
```

Bump `initialDelaySeconds` in the configuration & re-run to demonstrate how
pod ready latencies got increased