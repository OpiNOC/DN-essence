# DN-essence — Linee guida di sviluppo

## Obiettivo
Costruire un sistema Kubernetes-native per gestire rewrite DNS (CoreDNS) in modo dichiarativo, con una UI semplice per visualizzare e modificare le entry, evitando problemi di hairpin NAT senza dipendenze esterne.

Requisiti:
- leggero
- replicabile su più cluster
- GitOps-friendly
- zero modifica manuale del Corefile

---

## Architettura

Componenti:

1. CRD (DNSRewrite)
   - rappresenta una regola DNS

2. Controller (Go)
   - osserva le CRD
   - genera rewrite CoreDNS
   - aggiorna ConfigMap coredns
   - gestisce reload

3. API Backend (Go)
   - espone REST API
   - legge/scrive CRD
   - NON tocca CoreDNS

4. UI
   - CRUD entry DNS
   - vista semplice dominio → destinazione

---

## Modello dati

CRD DNSRewrite:

apiVersion: dns-essence.io/v1
kind: DNSRewrite
metadata:
  name: example
spec:
  host: api.miodominio.com
  target: ingress-nginx.default.svc.cluster.local
  enabled: true

Campi:
- host: FQDN pubblico
- target: service Kubernetes (FQDN interno)
- enabled: boolean

---

## Comportamento Controller

Responsabilità:
- watch DNSRewrite
- (opzionale) watch Ingress
- generare blocchi rewrite

Esempio output:

rewrite name api.miodominio.com ingress-nginx.default.svc.cluster.local

Azioni:
- patch ConfigMap coredns (kube-system)
- mantenere idempotenza
- deduplicare entry
- evitare override di config non gestita
- gestire errori e rollback

---

## Flusso operativo

1. Utente usa UI
2. UI chiama API
3. API crea/aggiorna CRD
4. Controller rileva cambiamento
5. Controller aggiorna CoreDNS
6. CoreDNS reload automatico

---

## UI Requirements

Funzionalità:
- lista entry DNS
  - host
  - target
  - stato (enabled/disabled)

- azioni:
  - create
  - update
  - delete
  - enable/disable

Validazioni:
- host deve essere FQDN valido
- target deve essere svc.cluster.local valido

UX:
- semplice
- niente concetti Kubernetes esposti
- focus su mapping dominio → destinazione

---

## API Backend

Endpoints:

GET /rewrites
POST /rewrites
PUT /rewrites/{name}
DELETE /rewrites/{name}

Comportamento:
- usa client Kubernetes (controller-runtime)
- lavora solo con CRD
- nessuna logica CoreDNS

---

## Sicurezza

RBAC:
- controller:
  - read/write DNSRewrite
  - read/write ConfigMap coredns

- API:
  - read/write DNSRewrite

Separazione:
- solo controller modifica CoreDNS

---

## Estensioni future

- wildcard (*.domain.com)
- auto-generazione da Ingress
- multi-namespace support
- audit log
- import/export config

---

## Testing

- unit test controller
- test idempotenza
- test aggiornamenti concorrenti
- test multi-entry

---

## Linee guida tecniche

- linguaggio: Go
- controller: kubebuilder / controller-runtime
- API: net/http o gin
- evitare complessità inutile
- codice leggibile e stabile

---

## Deployment

Deploy via:
- Helm (preferito)
- o Kustomize

Componenti:
- CRD
- controller
- API
- UI

---

## Filosofia

DN-essence deve:
- nascondere CoreDNS
- risolvere hairpin NAT in-cluster
- essere replicabile su molti cluster
- essere semplice da usare e mantenere

---

## Definition of Done

- UI con CRUD completo
- CRD funzionante
- controller aggiorna CoreDNS correttamente
- nessuna modifica manuale CoreDNS richiesta
- deploy replicabile su cluster multipli
