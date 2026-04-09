# Kubernetes self-hosting

Kubernetes deployment assets live here.

- Helm charts are under `self-hosting/kubernetes/helm-charts/`.
- CI workflows now read charts from this path.

## Examples

```bash
# lint umbrella chart
make -C self-hosting/kubernetes lint CHART=opentaco

# run helm-unittest if the chart has tests/
make -C self-hosting/kubernetes test CHART=taco-orchestrator
```
