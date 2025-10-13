# kube-burner Orchestration

This guide walks through the demo end to end: building the sample application, deploying it with kube-burner, exercising it with an exponential load generator, and observing metrics via direct port-forwards. The demo highlights how a cluster behaves under load, how it recovers when scaled, and how to capture the metrics that proves it.

---
## 1. Prerequisites
- Docker Desktop (or Docker Engine) with access to this repo directory.
- `kubectl` and either **kind** (recommended) or any Kubernetes cluster.
- `curl`, `tar`, and `bash` to download kube-burner.
- Clone the repository:
   ```bash
  git clone https://github.com/Praneeth-18/kube-burner-demo-for-software-engineers.git
  cd kube-burner-demo-for-software-engineers
  ```

---
## 2. Build the demo containers

```bash
docker build -t kind.local/demo-app-backend:latest images/backend

docker build -t kind.local/demo-app-frontend:latest images/frontend

docker build -t kind.local/demo-app-load:latest images/load-generator
```

Change the tags if you are using a different cluster/registry.

---
## 3. Create and prepare the cluster

For kind:

```bash
kind create cluster --name kube-burner-demo
kind load docker-image kind.local/demo-app-backend:latest --name kube-burner-demo
kind load docker-image kind.local/demo-app-frontend:latest --name kube-burner-demo
kind load docker-image kind.local/demo-app-load:latest --name kube-burner-demo
kubectl get nodes
```

Skip the `kind load` step if your cluster already has access to these images.

---
## 4. Install kube-burner

Download the binary matching your platform. 

(OR) 

Simply run "make build" in the root of the cloned [kube-burner](https://github.com/kube-burner/kube-burner) and copy the "bin" folder to this repo's root.

---
## 5. Configure the run

Edit `tmp/demo-user-data.yaml`—this file controls names, image tags, replica counts, load profile, and the UUID. Every field can be tweaked so you can dial in lighter or heavier runs without touching the config. 
**Prefer `.env` for load tweaks**
- Copy `.env.example` to `.env`, edit the variables you care about (baseline/load pauses, replicas, ramp knobs), then sync them into the YAML before running:
  ```bash
  cp .env.example .env
  # edit .env with your preferred editor
  ./scripts/sync-load-env.py
  ```
- Re-run the sync command whenever you change `.env`; values left unset fall back to whatever is already in the YAML.
- Use `BACKEND_SCALE_REPLICAS` and `LOAD_SCALE_DELAY` in `.env` if you want the automatic backend scale-up or load tear-down timing to change.
- Use `BACKEND_SCALE_REPLICAS` in `.env` if you want the automatic scale-up patch to target a different replica count.
- If you prefer, edit `tmp/demo-user-data.yaml` directly—the `.env` workflow is just a friendlier wrapper which keeps newcomers out of raw YAML while still letting you version different load profiles alongside the repo.

---

### Stress Load Profile for DEMO

Use these settings in `tmp/demo-user-data.yaml` to push the backend past its limits:

```yaml
uuid: demo-run-001
namespacePrefix: app-demo
appName: movie-night
backendName: demo-backend
backendServiceName: demo-backend
backendReplicas: 1                # start with one backend pod
frontendName: demo-frontend
frontendServiceName: demo-frontend
frontendReplicas: 1
loadGeneratorName: demo-load
loadGeneratorServiceName: demo-load
loadGeneratorReplicas: 1          
backendImage: kind.local/demo-app-backend:latest
frontendImage: kind.local/demo-app-frontend:latest
loadGeneratorImage: kind.local/demo-app-load:latest
frontendPublicUrl: http://localhost:8081
loadGeneratorActions: book_ticket,cancel_ticket,give_feedback
enableLoad: true
baselinePause: 20s
loadPause: 30s
loadGeneratorBaseRps: "10"
loadGeneratorRampFactor: "5"
loadGeneratorRampIntervalSeconds: "5"
loadGeneratorRunDurationSeconds: "90"
prometheusName: demo-prometheus
prometheusServiceName: demo-prometheus
prometheusImage: prom/prometheus:v2.54.0
```

Key knobs exposed through `.env` and the metadata file:
- `uuid`: differentiates each run and namespace.
- `backendScaleReplicas`: backend replica count after the automated scale-up.
- `baselinePause` / `loadPause`: waiting periods before and during the load job.
- `loadGenerator*` variables: exponential ramp profile and runtime (defaults give a 90 s burst).
- `loadScaleDelay`: how long kube-burner waits after scaling the backend before removing the load generator deployment.
- Image tags, service names, and Prometheus configuration.




Remember to change the `uuid` before each run; the namespace will be `${namespacePrefix}-${uuid}` (e.g. `app-demo-demo-run-001`).








### Ramp pattern (You can tweak these as per your desired load numers in `tmp/demo-user-data.yaml`
- Starts at `BASE_RPS` requests per second.
- Every `RAMP_INTERVAL_SECONDS`, multiplies the rate by `RAMP_FACTOR`.
  * Example with defaults (`BASE_RPS=2`, `RAMP_FACTOR=1.35`, `RAMP_INTERVAL_SECONDS=45`):
    - 0 seconds: ~2 rps (2 × 1.35^0)
    - 45 seconds: ~2.7 rps (2 × 1.35^1)
    - 90 seconds: ~3.6 rps (2 × 1.35^2)
    - 135 seconds: ~4.9 rps (2 × 1.35^3)
    - 180 seconds: ~6.6 rps (2 × 1.35^4)



> Tip: keep multiple copies of the file (for example `demo-user-data-light.yaml`, `demo-user-data-stress.yaml`) and copy the one you want to `tmp/demo-user-data.yaml` before each run.

---
## 6. Run the demo

```bash
./bin/kube-burner init \
  -c config/demo-run.yml \
  --user-data tmp/demo-user-data.yaml \
  --uuid demo-run-001
```

What happens during the run:
1. kube-burner pre-pulls images via a DaemonSet for faster pod start-up.
2. The backend, frontend, and Prometheus deploy; `baselinePause` gives you time to poke around.
3. The load generator deployment starts and runs for `loadPause` + `RUN_DURATION_SECONDS`.
4. After `loadPause`, kube-burner patches the backend to `backendScaleReplicas` (default 2) so you can watch recovery.
5. `loadScaleDelay` seconds later, kube-burner deletes the load generator deployment, letting traffic wind down automatically.

Adjust the delays or replica targets in `.env` to explore different stories (longer spikes, later recovery, etc.).

---
## 7. Observe the system via port-forward 

While the namespace exists (`app-demo-<uuid>`):


- **Frontend UI**
  ```bash
  kubectl port-forward svc/demo-frontend -n app-demo-demo-run-001 8080:80
  ```
  Visit <http://localhost:8080/> to watch counters update.

- **Backend metrics**
  ```bash
  kubectl port-forward svc/demo-backend -n app-demo-demo-run-001 8081:8080
  curl http://localhost:8081/metrics | grep app_interactions_total
  ```

- **Prometheus UI**
  ```bash
  kubectl port-forward svc/demo-prometheus -n app-demo-demo-run-001 9090:9090
  ```
  Browse <http://localhost:9090> to explore the baked-in dashboards or run ad-hoc queries to interpret the metrics (e.g., **app_interactions_total, app_active_sessions, http_request_duration_seconds_bucket, lg_sent_requests_total**).

##### OPTIONAL:

- **Load generator metrics (during load window)**
  ```bash
  kubectl port-forward svc/demo-load -n app-demo-demo-run-001 2112:2112
  curl http://localhost:2112/metrics | grep lg_
  ```

These endpoints expose everything you need: HTTP server metrics, interaction counts, load-generator gauges/counters, and the in-cluster Prometheus scrape results.

**Prometheus queries worth trying**
- `rate(app_interactions_total[1m])`
- `rate(lg_errors_total[1m])`
- `histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[2m])) by (le))`
- `histogram_quantile(0.99, sum(rate(lg_request_duration_seconds_bucket[2m])) by (le))`
- `sum(rate(http_request_duration_seconds_bucket{le="0.5"}[5m])) / sum(rate(http_request_duration_seconds_count[5m]))`
- `lg_current_rps`
- `histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket{namespace="app-demo-demo-run-001"}[5m])) by (le))`
- `histogram_quantile(0.95, sum(rate(lg_request_duration_seconds_bucket{namespace="app-demo-demo-run-001"}[5m])) by (le))`
- `sum(increase(kubelet_pod_worker_duration_seconds_count{namespace="app-demo-demo-run-001"}[10m]))`

Latency-focused queries (replace `<uuid>` with your run, for example `demo-run-001`):
- `histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket{namespace="app-demo-<uuid>"}[5m])) by (le))` – backend request latency (p95).
- `histogram_quantile(0.95, sum(rate(lg_request_duration_seconds_bucket{namespace="app-demo-<uuid>"}[5m])) by (le))` – load generator request latency (p95).
- `sum(increase(kubelet_pod_worker_duration_seconds_count{namespace="app-demo-<uuid>"}[10m]))` – kubelet pod-startup counts/latency.

These show throughput, backend latency percentiles, and load-generator behavior.


---

### Why kube-burner at all?

It gives you a repeatable script for:

- Creating the whole stack (frontend + backend + Prometheus + configurable load) with one command.
- Versioning the load profile (tmp/demo-user-data.yaml).
- Re-running the same sequence with different load parameters just by editing the YAML.
- Automatically collecting pod latency and service metrics.

Without kube-burner you’d have to kubectl apply a bunch of manifests, sync the timers manually, and clean up by hand.

---

### Some commands to play with:

1. Recover by scaling the backend: 
   ```bash
   kubectl scale deployment demo-backend -n app-demo-demo-run-001 --replicas=3
   ```
2. When finished, stop the load generator but leave the app running: 
   ```bash
    kubectl scale deployment demo-load -n app-demo-demo-run-001 --replicas=0
    ```

3. To see the pods live:
```bash
  kubectl get pods -n app-demo-demo-run-001
```


You only need to rerun kube-burner with a new UUID when you want a completely separate namespace (for example, `demo-run-002`) so you can compare runs side-by-side.




---

## 8. Experiment

- Duplicate `.env` to create “light” and “stress” profiles (e.g., raise `LOAD_BASE_RPS`, `LOAD_RAMP_FACTOR`, or shorten `LOAD_RAMP_INTERVAL_SECONDS`).

- Bump `backendScaleReplicas` to see how many pods are needed to recover.

- Increase `loadScaleDelay` to keep the load running longer before the deployment is removed.

- Rerun with a new `uuid` so you can compare namespaces side by side (`app-demo-demo-run-001`, `app-demo-demo-run-002`, …).

--- 
## 9. Cleanup

```bash
./bin/kube-burner destroy --uuid demo-run-001
kubectl delete namespace app-demo-demo-run-001 --ignore-not-found
```

Optional:

```bash
kind delete cluster --name kube-burner-demo
```

Before the next run, update the `uuid` in `demo-user-data.yaml` (and copy/sync a new `.env` profile if desired).

---

## 10. Troubleshooting
- **Image pull errors**: ensure the images were loaded into kind or pushed to a reachable registry.
- **Load looks too short**: increase `loadPause`, `loadScaleDelay`, or `LOAD_RUN_DURATION_SECONDS`.
- **UUID already exists**: run `./bin/kube-burner destroy --uuid <old>` or choose a fresh UUID.
- **No backend scale-up**: verify `BACKEND_SCALE_REPLICAS` is greater than the initial replica count and that `.env` changes were synced.
- **Metrics missing**: confirm the port-forward sessions are active, and that Prometheus is scraping the services (labels set in the templates).

---

# Here is the full walkthrough with the default settings so a newcomer can follow the entire lifecycle and know exactly what happens when.

### Initial setup

You start with `.env` values synced into `tmp/demo-user-data.yaml`. Key defaults:
*   `baselinePause=20s`
*   `loadPause=30s`
*   `LOAD_RUN_DURATION_SECONDS=90`
*   `BACKEND_SCALE_REPLICAS=2`
*   `LOAD_SCALE_DELAY=30s`

You launch the scenario with:
```bash
./bin/kube-burner init -c config/demo-run.yml --user-data tmp/demo-user-data.yaml --uuid demo-run-001
```

### Phase 1 – Baseline stack (0–20 s)

1.  `kube-burner` pre-pulls images so workloads start quickly.
2.  It creates the backend API (1 replica), the static frontend, and Prometheus in namespace `app-demo-demo-run-001`.
3.  After the pods become ready, `kube-burner` waits for `baselinePause` (20 s). This quiet window lets you click through the frontend and confirm everything is healthy before load starts.

### Phase 2 – Load ramp (20–50 s)

4.  The `start-load-generator` job creates the load deployment (1 replica). Once the pod is ready, `kube-burner` waits for `loadPause` (30 s).
5.  During this 30 s window, the Python client runs: it ramps exponentially with `BASE_RPS=10`, `RAMP_FACTOR=5`, `RAMP_INTERVAL_SECONDS=5`, and keeps firing requests for `RUN_DURATION_SECONDS=90`. Because it’s a Deployment, Kubernetes will restart the pod if it exits early; the delete job later ensures it stops for good.

### Phase 3 – Automatic backend recovery (≈50 s)

6.  Immediately after `loadPause` expires, `kube-burner` triggers the `scale-backend patch` job. This sets the backend Deployment to `BACKEND_SCALE_REPLICAS` (2 replicas by default). Within a few seconds you’ll see a second backend pod spin up and request load should recover.

### Phase 4 – Observe the scaled state (50–80 s)

7.  `kube-burner` pauses again for `LOAD_SCALE_DELAY` (30 s). This gives you time to watch Prometheus charts and confirm the new backend capacity is handling the traffic.

### Phase 5 – Tear down the load (≈80 s)

8.  After that 30 s observation window, the `stop-load-generator` job runs with `jobType: delete`. It looks up resources labeled as part of the load job (`kube-burner-job=start-load-generator`) and deletes the deployment. With no controller keeping it alive, the load generator pods vanish and traffic drops to zero.

### End state

9.  `kube-burner` exits cleanly. The namespace still contains the frontend, scaled backend, and Prometheus; the load deployment is gone.
10. You can continue to observe metrics or recycle the namespace. For the next demo, adjust `.env`, re-run `./scripts/sync-load-env.py`, pick a fresh `uuid`, and repeat.

This flow—quiet baseline, controlled spike, automated recovery, and deliberate teardown—gives a clear, repeatable story you can show to anyone learning `kube-burner` or cluster performance tuning.


