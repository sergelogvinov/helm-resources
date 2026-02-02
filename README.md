# Helm Resources Plugin

A Helm plugin that displays resource requests and limits for all workloads in a helm release.
It also shows current resource usage and provides recommendations for under-provisioned containers.

## Motivation

I deploy applications using Helm charts in most cases, so it is crucial to ensure that resource requests and limits are properly configured for each workload.
This allows the Kubernetes scheduler to make better decisions about how to distribute workloads across cluster nodes.
It also helps node autoscalers or Karpenter, to scale the cluster based on actual resource usage.

However, application code is constantly evolving: new features are added, and resource consumption patterns change over time.
What was an appropriate configuration for resource requests and limits a month ago may no longer be sufficient today.

This plugin helps identify such issues by comparing real-time resource usage with the configured requests and limits,
and provides recommendations for adjusting them when necessary.

## Installation

```bash
helm plugin install https://github.com/sergelogvinov/helm-resources
```

## Usage

```bash
# Show resources for a release in the current namespace
helm resources my-release

# Show resources for a release in a specific namespace
helm resources my-release --namespace production

# Use external Prometheus server for metrics
helm resources my-release --prometheus-url http://prometheus.monitoring.svc.cluster.local:9090

# Use maximum metrics instead of average
helm resources my-release --prometheus-url http://prometheus:9090 --aggregation max

# Use custom time window with average metrics
helm resources my-release --prometheus-url http://prometheus:9090 --aggregation avg --metrics-window 1h
```

### Output Formats

```bash
# Default table format
helm resources my-release

# JSON output
helm resources my-release -o json

# YAML output
helm resources my-release -o yaml
```

### Example Output

**Table format:**

```
KIND         NAME            REPLICAS  CONTAINER     REQUESTS (CPU/Memory)  LIMITS (CPU/Memory)   USAGE (CPU/Memory)
Deployment   web-server      2         nginx         100m/128Mi             500m/512Mi            85m/95Mi
Deployment   web-server      2         sidecar       50m/64Mi               100m/128Mi            35m/48Mi
StatefulSet  database        2         postgres      200m/256Mi             1/1Gi                 320m/400Mi
DaemonSet    log-collector   3         fluentd       50m/64Mi               200m/256Mi            42m/58Mi

RESOURCE RECOMMENDATIONS:

KIND        NAME      CONTAINER  CURRENT (CPU/Memory)  RECOMMENDED (CPU/Memory)
StatefulSet database  postgres   200m/256Mi            384m/480Mi
```

**Resource Analysis:**

The plugin automatically analyzes resource usage vs requests and provides recommendations when:
- Container usage exceeds resource requests
- Recommendations suggest 20% buffer above current usage

## Metrics Sources

The plugin can fetch current resource usage metrics from Prometheus.

```bash
helm resources my-app --prometheus-url http://prometheus.monitoring.svc.cluster.local:9090
helm resources my-app --prometheus-url https://prometheus.example.com --aggregation avg --metrics-window 1h
helm resources my-app --prometheus-url http://prometheus:9090 --aggregation max --metrics-window 24h
```

### Aggregation Options

- `avg` - Average metrics (default) - better for typical usage patterns
- `max` - Maximum metrics - better for capacity planning and peak analysis

### Metrics Window Options

- `5m` - 5 minutes (default)
- `15m` - 15 minutes
- `1h` - 1 hour
- `6h` - 6 hours
- `24h` - 24 hours
- `7d` - 7 days

## Requirements

- Helm v3+
- Kubernetes cluster access
