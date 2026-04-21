# DN-essence

**Kubernetes-native CoreDNS rewrite manager.**

DN-essence lets you manage CoreDNS DNS rewrite rules declaratively, without ever editing the Corefile by hand. It solves the classic [hairpin NAT](https://en.wikipedia.org/wiki/Network_address_translation#Hairpinning) problem: pods that need to reach a public domain name that resolves to a service running inside the same cluster.

Rules are stored as Kubernetes Custom Resources (`DNSRewrite`). A controller watches them and keeps the CoreDNS ConfigMap in sync automatically. A web UI and a REST API let you manage rules without touching `kubectl`.

---

## How it works

```
User (UI or kubectl)
        │
        ▼
  DNSRewrite CRD   ◄──────────────────────────────┐
        │                                          │
        ▼                                          │
  Controller                               single source
        │                                   of truth
        ▼
CoreDNS ConfigMap (kube-system)
        │
        ▼
   CoreDNS reload
```

The controller manages only a clearly delimited section of the Corefile, leaving all existing configuration untouched:

```
.:53 {
    # ... your existing CoreDNS config (never modified) ...

    # BEGIN dn-essence
    rewrite name api.example.com myapp.default.svc.cluster.local
    # END dn-essence
}
```

---

## Prerequisites

- Kubernetes cluster with CoreDNS (standard kubeadm/k3s/kind setup)
- CoreDNS `reload` plugin enabled (default in all standard distributions)
- Helm 3

---

## Installation

### From the OCI Helm chart (recommended)

```bash
helm install dn-essence oci://ghcr.io/opinoc/helm-charts/dn-essence \
  --namespace dn-essence \
  --create-namespace
```

### From source

```bash
git clone https://github.com/OpiNOC/DN-essence.git
helm install dn-essence ./DN-essence/deploy/helm/dn-essence \
  --namespace dn-essence \
  --create-namespace
```

### Verify the installation

```bash
kubectl rollout status deployment/dn-essence -n dn-essence
kubectl get dnsrewrites
```

---

## Configuration

All options are set via Helm values:

| Value | Default | Description |
|-------|---------|-------------|
| `image.repository` | `ghcr.io/opinoc/dn-essence` | Container image |
| `image.tag` | `latest` | Image tag |
| `image.pullPolicy` | `Always` | Pull policy |
| `coredns.namespace` | `kube-system` | Namespace of the CoreDNS ConfigMap |
| `coredns.configmap` | `coredns` | Name of the CoreDNS ConfigMap |
| `httpPort` | `9090` | Port for the API and UI server |
| `healthPort` | `8081` | Port for health/readiness probes |
| `service.type` | `ClusterIP` | Kubernetes Service type |
| `replicaCount` | `1` | Number of replicas |

Example — expose the UI via NodePort:

```bash
helm upgrade dn-essence oci://ghcr.io/opinoc/helm-charts/dn-essence \
  --namespace dn-essence \
  --set service.type=NodePort
```

---

## Usage

### Web UI

Port-forward the service and open your browser:

```bash
kubectl port-forward svc/dn-essence 8080:80 -n dn-essence
# Open http://localhost:8080
```

The UI lets you create, edit, enable/disable, and delete rules. Status updates automatically — you don't need to refresh the page after a change.

### REST API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/rewrites` | List all rules |
| `POST` | `/api/rewrites` | Create a rule |
| `PUT` | `/api/rewrites/{name}` | Update a rule |
| `DELETE` | `/api/rewrites/{name}` | Delete a rule |

Example:

```bash
curl -X POST http://localhost:8080/api/rewrites \
  -H 'Content-Type: application/json' \
  -d '{"name":"my-app","host":"myapp.example.com","target":"myapp.default.svc.cluster.local","enabled":true}'
```

### kubectl

Rules created from the UI or API are immediately visible via `kubectl`, and vice versa. The Kubernetes API is the single source of truth.

**Create a rule:**

```bash
kubectl apply -f - <<EOF
apiVersion: dns-essence.io/v1
kind: DNSRewrite
metadata:
  name: my-app
spec:
  host: myapp.example.com
  target: myapp.default.svc.cluster.local
  enabled: true
EOF
```

**List all rules:**

```bash
kubectl get dnsrewrites
```

**Disable a rule without deleting it:**

```bash
kubectl patch dnsrewrite my-app --type merge -p '{"spec":{"enabled":false}}'
```

**Delete a rule:**

```bash
kubectl delete dnsrewrite my-app
```

---

## DNSRewrite reference

```yaml
apiVersion: dns-essence.io/v1
kind: DNSRewrite
metadata:
  name: my-app          # unique identifier (Kubernetes name conventions)
spec:
  host: myapp.example.com                          # public FQDN to rewrite
  target: myapp.default.svc.cluster.local          # in-cluster service FQDN
  enabled: true                                    # false = rule kept but inactive
status:
  applied: true         # set by the controller when the rule is live in CoreDNS
  error: ""             # last reconciliation error, if any
```

---

## Architecture

| Component | Description |
|-----------|-------------|
| **CRD** (`DNSRewrite`) | Cluster-scoped custom resource. The only place rules are stored. |
| **Controller** | Watches `DNSRewrite` resources, patches the CoreDNS ConfigMap, updates status. |
| **API server** | REST API that reads and writes `DNSRewrite` CRDs. Never touches CoreDNS directly. |
| **UI** | Vanilla JS single-page app, embedded in the Go binary via `embed.FS`. |

Everything runs in a single binary and a single container.

---

## Uninstall

```bash
helm uninstall dn-essence -n dn-essence

# Remove CRDs and all rules (irreversible)
kubectl delete crd dnsrewrites.dns-essence.io
```

> **Note:** uninstalling does not automatically clean up the `# BEGIN dn-essence` block from the CoreDNS ConfigMap. Remove it manually if needed, or re-install and delete all rules first.

---

## Development

```bash
git clone https://github.com/OpiNOC/DN-essence.git
cd DN-essence

# Run tests
go test ./...

# Run locally against your current kubeconfig cluster
HTTP_ADDR=:9090 HEALTH_ADDR=:9091 go run ./cmd/manager/
```

The controller connects to whatever cluster your current `kubeconfig` points to. The CoreDNS namespace and ConfigMap name can be overridden with `COREDNS_NAMESPACE` and `COREDNS_CONFIGMAP` env vars.

---

## License

[MIT](LICENSE)
