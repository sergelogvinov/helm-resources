# Helm Resources Plugin

This plugin helps you check resource requests and limits for all workloads in a Helm release.
It shows current resource usage and gives recommendations if containers need more resources.

## Why Use This Plugin

When you deploy applications with Helm charts, it is important to set proper resource requests and limits.
This helps the Kubernetes scheduler place workloads on the right nodes.
It also helps tools like node autoscalers or Karpenter scale your cluster based on actual usage.

Applications change over time. What worked a month ago might not be enough today.
This plugin compares real-time usage with your current settings and tells you when to update them.

## Installation

```shell
helm plugin install https://github.com/sergelogvinov/helm-resources
```

## Basic Usage

```shell
# Show resources for a release in the current namespace
helm resources my-release

# Show resources for a release in a specific namespace
helm resources my-release --namespace production

# Use external Prometheus server for metrics
helm resources my-release --prometheus-url http://prometheus.monitoring.svc.cluster.local:9090
```

### Apply Resource Recommendations

You can automatically update your Helm values file with resource recommendations:

```shell
# Apply recommendations to a values file
helm resources my-release --prometheus-url http://prometheus:9090 --values values.yaml

# Apply to multiple values files
helm resources my-release --prometheus-url http://prometheus:9090 --values overrides.yaml --values base.yaml

# Use specific aggregation settings
helm resources my-release --prometheus-url http://prometheus:9090 --aggregation max --metrics-window 1h --values values.yaml
```

What the plugin does:
- Gets current resource usage from Prometheus or Kubernetes
- Calculates recommended requests and limits with a safety margin
- Updates your values file with the new settings
- Keeps your existing YAML structure and comments
- Adds `resources` sections where they are missing

**Real-world usage example:**

```shell
$ git status
On branch main
Your branch is up to date with 'origin/main'.

nothing to commit, working tree clean

$ helm resources -n production backend -f back/.helm/values.backend.override.yaml -f back/.helm/values.backend.yaml --show-stats=false
KIND        NAME               CONTAINER            REQUESTS (CPU/MEM)  REQUESTS DIFF (%)  LIMITS (CPU/MEM)  LIMITS DIFF (%)  USAGE (CPU/MEM)
Deployment  backend-celer      backend-celery       200m/-              +100%/-            -                 -                131m/485Mi
Deployment  backend-ticket     backend-ticket       -/2.0Gi             -/+28%             -/3.2Gi           -/+62%           8m/1.6Gi
Deployment  backend-payment    backend-payment      -/512Mi             -/+33%             -                 -                3m/392Mi

$ git --no-pager diff
diff --git a/back/.helm/values.backend.yaml b/back/.helm/values.backend.yaml
index 3cfe35e3f8d..33c882d7195 100644
--- a/back/.helm/values.backend.yaml
+++ b/back/.helm/values.backend.yaml
@@ -763,7 +763,7 @@ workers:
         cpu: 1
         memory: 2Gi
       requests:
-        cpu: 100m
+        cpu: 200m
         memory: 1Gi
@@ -994,7 +994,7 @@ workers:
         memory: 1Gi
       requests:
         cpu: 50m
-        memory: 384Mi
+        memory: 512Mi
@@ -1135,10 +1135,10 @@ workers:
     resources:
       limits:
         cpu: 300m
-        memory: 2Gi
+        memory: 3Gi
       requests:
         cpu: 100m
-        memory: 1600Mi
+        memory: 2Gi
```

**Supported values.yaml structures:**

```yaml
# Top-level resources
resources:
  limits:
    cpu: 1
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 256Mi

# Specific workload with resources
backup:
  ...

  resources:
    limits:
      cpu: 1
      memory: 512Mi
    requests:
      cpu: 100m
      memory: 256Mi

# With sidecar containers, e.g., for a metrics exporter
metrics:
  ...

  resources:
    limits:
      cpu: 1
      memory: 512Mi
    requests:
      cpu: 100m
      memory: 256Mi

# With SubSection named as `services`, `workers`, or `jobs`
services:
  web-server:
    containers:
      - name: nginx
        image: nginx:latest
        resources:
          limits:
            cpu: 127m
            memory: 142Mi
          requests:
            cpu: 102m
            memory: 114Mi
```

The plugin finds the right place in your values file based on the workload name and structure.

**After applying recommendations:**

```yaml
services:
  web-server:
    containers:
      - name: nginx
        image: nginx:latest
        resources:
          limits:
            cpu: 1
            memory: 512Mi
          requests:
            cpu: 100m
            memory: 256Mi
```

### Output Formats

```shell
# Default table format (human-readable)
helm resources my-release

# JSON output
helm resources my-release -o json

# YAML output
helm resources my-release -o yaml
```

### Example Output

**Table format:**

```shell
KIND         NAME                     REPLICAS  CONTAINER                       REQUESTS (CPU/MEM)  LIMITS (CPU/MEM)  USAGE (CPU/MEM)
StatefulSet  pg-backend               1         pg-backend                      100m/4.0Gi          2.0/10.0Gi        139m/1.1Gi
StatefulSet  pg-backend               1         metrics                         10m/32Mi            200m/128Mi        -/10Mi
CronJob      pg-backend-backup-check  0         postgresql-single-backup-check  100m/512Mi          2.0/1.0Gi         -
CronJob      pg-backend-backup        0         postgresql-single-backup        1.5/768Mi           2.0/4.0Gi         -

Resource recommendations to adjust:

KIND         NAME        CONTAINER   REQUESTS (CPU/MEM)  REQUESTS DIFF (%)  LIMITS (CPU/MEM)  LIMITS DIFF (%)  USAGE (CPU/MEM)
StatefulSet  pg-backend  pg-backend  200m/-              +100%/-            -                 -                139m/1.1Gi
```

**Resource Analysis:**

The plugin checks if containers need more resources. It gives recommendations when:
- Container usage is higher than the requested resources
- The recommended resources are 20% more than current usage

## Metrics Sources

The plugin gets current resource usage from Prometheus.

```shell
# Use Prometheus server in the cluster
helm resources my-app --prometheus-url http://prometheus.monitoring.svc.cluster.local:9090

# With aggregation and metrics window options
helm resources my-app --prometheus-url https://prometheus.example.com --aggregation avg --metrics-window 1h
```

### Aggregation Options

- `avg` - Average metrics (default) - shows typical usage
- `max` - Maximum metrics - shows peak usage

### Metrics Window Options

- `5m` - 5 minutes (default)
- `15m` - 15 minutes
- `1h` - 1 hour
- `6h` - 6 hours
- `24h` - 24 hours
- `7d` - 7 days

### Output Formatting Flags

- `--show-stats` - Show resource statistics (default: true)
- `--show-recommendations` - Show resource recommendations (default: true)
- `--no-headers` - Hide table headers

### Environment Variables

You can set these environment variables instead of using flags:

- `PROMETHEUS_URL` - Prometheus server URL (e.g., http://prometheus:9090)
- `METRICS_WINDOW` - Time window for queries (e.g., 5m, 1h, 24h)
- `AGGREGATION` - How to aggregate metrics (avg or max)

**Example with environment variables:**

```shell
# Set environment variables
export PROMETHEUS_URL=http://prometheus.monitoring.svc.cluster.local:9090
export METRICS_WINDOW=1h
export AGGREGATION=max

helm resources my-release

# Or set all at once
PROMETHEUS_URL=http://prometheus:9090 METRICS_WINDOW=1h AGGREGATION=max helm resources my-release
```

## Requirements

- Helm v3+
- Kubernetes cluster access
