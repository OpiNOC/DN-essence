# DN-essence — Project reference

## Goal

Build a Kubernetes-native system to manage CoreDNS rewrite rules declaratively, with a simple UI to view and edit entries, solving the hairpin NAT problem without external dependencies.

Requirements:
- Lightweight
- Replicable across multiple clusters
- GitOps-friendly
- Zero manual Corefile edits

---

## Architecture

### Components

1. **CRD (`DNSRewrite`)** — cluster-scoped custom resource representing a DNS rewrite rule
2. **Controller (Go)** — watches DNSRewrite CRDs, patches CoreDNS ConfigMap, updates status
3. **API backend (Go)** — REST API that reads/writes CRDs; never touches CoreDNS directly
4. **UI (vanilla JS)** — embedded in the Go binary via `embed.FS`

Everything runs in a **single binary and a single container**.

### Data flow

```
User (UI or kubectl)
        │
        ▼
  DNSRewrite CRD   ◄── single source of truth
        │
        ▼
  Controller (reconcile loop)
        │
        ▼
CoreDNS ConfigMap (kube-system)
        │
        ▼
   CoreDNS auto-reload (~30s)
```

---

## Repository structure

```
DN-essence/
├── api/v1/
│   ├── dnsrewrite_types.go       # CRD Go types (Spec, Status, List)
│   ├── groupversion_info.go      # API group registration
│   └── zz_generated.deepcopy.go # DeepCopy implementations
├── cmd/manager/
│   └── main.go                   # Single entrypoint: manager + API server
├── internal/
│   ├── controller/
│   │   └── dnsrewrite_controller.go  # Reconcile loop
│   ├── coredns/
│   │   ├── configmap.go              # CoreDNS ConfigMap patch logic
│   │   └── configmap_test.go         # Unit tests
│   └── api/
│       └── handler.go                # HTTP handlers (4 endpoints)
├── ui/
│   ├── embed.go                  # go:embed declaration
│   └── dist/
│       ├── index.html
│       ├── style.css
│       └── app.js
├── config/crd/
│   └── dns-essence.io_dnsrewrites.yaml  # CRD manifest
├── deploy/helm/dn-essence/       # Helm chart
│   ├── Chart.yaml
│   ├── values.yaml
│   ├── crds/                     # CRD installed by Helm before templates
│   └── templates/
│       ├── _helpers.tpl
│       ├── deployment.yaml
│       ├── service.yaml
│       ├── rbac.yaml             # ClusterRole + Role (kube-system) + bindings
│       └── serviceaccount.yaml
├── .github/workflows/
│   └── build.yaml                # CI: test → build image → publish Helm chart
├── Dockerfile                    # Multi-stage: golang:1.25-alpine + distroless
├── go.mod
└── go.sum
```

---

## Data model

```yaml
apiVersion: dns-essence.io/v1
kind: DNSRewrite
metadata:
  name: my-app
spec:
  host: api.example.com                       # public FQDN
  target: myapp.default.svc.cluster.local     # in-cluster service FQDN
  enabled: true                               # false = inactive but preserved
status:
  applied: true    # set by controller when written to CoreDNS
  error: ""        # last reconciliation error, if any
```

---

## Controller behavior

The reconcile loop is triggered on any DNSRewrite create/update/delete:

1. List all `DNSRewrite` resources cluster-wide
2. Filter `enabled: true` entries, build sorted rewrite lines
3. Fetch the CoreDNS ConfigMap
4. Compare current managed block with desired state — **no-op if already up-to-date** (idempotency)
5. Patch only the marked section; leave all other config untouched
6. Update `.status.applied` and `.status.error` on every CRD

### CoreDNS ConfigMap — managed block

Only the section between the markers is ever written:

```
.:53 {
    # ... existing config, never touched ...

    # BEGIN dn-essence
    rewrite name api.example.com myapp.default.svc.cluster.local
    # END dn-essence
}
```

CoreDNS detects the ConfigMap change and reloads automatically via the `reload` plugin (present in all standard distributions). If the new config is invalid, CoreDNS keeps the previous configuration.

---

## API

| Method | Path | Action |
|--------|------|--------|
| `GET` | `/api/rewrites` | List all DNSRewrite resources |
| `POST` | `/api/rewrites` | Create a new DNSRewrite |
| `PUT` | `/api/rewrites/{name}` | Update an existing DNSRewrite |
| `DELETE` | `/api/rewrites/{name}` | Delete a DNSRewrite |

Implemented with `net/http` standard library. Uses the controller-runtime client shared with the manager — no separate Kubernetes client.

---

## RBAC

Two separate roles:

**ClusterRole** (cluster-scoped, for DNSRewrite CRDs):
- `dns-essence.io/dnsrewrites`: get, list, watch, create, update, patch, delete
- `dns-essence.io/dnsrewrites/status`: get, update, patch

**Role** (namespace-scoped, `kube-system`, for CoreDNS ConfigMap):
- `configmaps`: list, watch (required by controller-runtime cache; cannot use resourceNames)
- `configmaps` (name: `coredns`): get, update, patch

---

## Ports

| Port | Purpose |
|------|---------|
| `9090` | API + UI HTTP server (`HTTP_ADDR`) |
| `8080` | controller-runtime metrics (Prometheus) |
| `8081` | Health/readiness probes (`HEALTH_ADDR`) |

---

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `COREDNS_NAMESPACE` | `kube-system` | Namespace of the CoreDNS ConfigMap |
| `COREDNS_CONFIGMAP` | `coredns` | Name of the CoreDNS ConfigMap |
| `HTTP_ADDR` | `:9090` | Bind address for API + UI |
| `HEALTH_ADDR` | `:8081` | Bind address for health probes |
| `DEBUG` | `false` | Enable verbose controller-runtime logging |

---

## CI/CD

**GitHub Actions** (`.github/workflows/build.yaml`):

1. **Test** — `go test ./...`
2. **Build & push image** — multi-platform Docker image → `ghcr.io/opinoc/dn-essence:latest` (and SHA tag)
3. **Publish Helm chart** — OCI chart → `ghcr.io/opinoc/helm-charts/dn-essence`

Triggered on every push to `main` and on version tags (`v*`).

---

## Technical guidelines

- Language: Go
- Controller framework: `sigs.k8s.io/controller-runtime`
- HTTP: `net/http` standard library
- UI: vanilla JS, no framework, embedded via `go:embed`
- Avoid unnecessary complexity
- Readable and stable code

---

## Future extensions

- Wildcard support (`*.domain.com`)
- Auto-generation from Ingress resources
- Multi-namespace support
- Audit log
- Import/export config

---

## Definition of Done

- [x] UI with full CRUD
- [x] CRD working in cluster
- [x] Controller correctly updates CoreDNS
- [x] No manual CoreDNS edits required
- [x] Replicable deployment via Helm
- [x] CI/CD pipeline (GitHub Actions → ghcr.io)
