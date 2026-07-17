# Microservices Platform

A production-grade-style microservices platform built for local, zero-cost learning: three polyglot services, RabbitMQ for async communication, and Traefik as the gateway. Observability, GitOps, and AI-assisted operations land in later phases.

## Stack

| Service | Language | Datastore | Port (compose) |
|---|---|---|---|
| user-service | Node.js + Express | MongoDB | 3001 |
| order-service | Python + FastAPI | PostgreSQL | 3002 |
| product-service | Go + Gin | Redis | 3003 |

Async messaging: RabbitMQ. Gateway: Traefik. Local Kubernetes: Kind.

## Prerequisites

Docker, kubectl, kind, Helm, Node 20+, Python 3.12+, Go 1.22+. See `SETUP.md` for install commands per OS.

## Quickstart — Docker Compose

```bash
cp .env.example .env
make up
```

Check every service is healthy:

```bash
curl http://localhost:3001/health   # user-service
curl http://localhost:3002/health   # order-service
curl http://localhost:3003/health   # product-service
curl http://localhost/api/users     # through the Traefik gateway
```

- Traefik dashboard: http://localhost:8081
- RabbitMQ management UI: http://localhost:15672 (guest/guest)

Tear down:

```bash
make down       # stop containers
make clean      # stop + remove volumes (wipes data)
```

## Local Kubernetes — Kind

```bash
make kind-up
kubectl get nodes
```

This gives you a 1 control-plane + 2 worker node cluster with ports 80/443 mapped, ready for an ingress controller. Helm charts and Kubernetes manifests land in Phase 2 — right now this just proves the cluster comes up cleanly.

## Roadmap

- [x] **Phase 1** — Repo scaffolding & runnable skeleton
- [x] **Phase 2** — Real service logic (JWT auth, order events, inventory) + Helm charts
- [x] **Phase 3** — CI/CD: GitHub Actions (build/test/scan/push/deploy) + reference Jenkinsfile
- [x] **Phase 4a** — Observability: Prometheus, Grafana, custom business metrics
- [x] **Phase 4b** — Observability: Loki + Grafana Alloy for centralized logging
- [ ] **Phase 5** — GitOps: Argo CD, Blue/Green → Canary rollouts
- [ ] **Phase 6** — AI integration: Ollama log analysis, smart alerting, self-healing
- [ ] **Phase 7** — Production hardening: NetworkPolicies, PDBs, HPA, backups

## Observability (Phase 4a)

All three services expose Prometheus metrics at `/metrics` — golden signals (request rate, error rate, latency) plus one business metric each: `user_registrations_total`, `orders_created_total`, and `product_stock_level` / `inventory_decrements_total`.

**Install the stack** (requires the Kind cluster + your services already deployed via `make helm-install`):
```bash
make observability-install
```
This deploys `kube-prometheus-stack` (Prometheus, Grafana, Alertmanager) into a new `monitoring` namespace, plus two alert rules and a pre-built Grafana dashboard.

**View Grafana:**
```bash
make grafana-forward
```
Then open `http://localhost:3000` — login `admin` / `admin`. The "Microservices Platform" dashboard is auto-imported; find it under Dashboards.

**View Prometheus:** the exact service name has a chart-generated suffix, so look it up first:
```bash
make observability-status
kubectl port-forward -n monitoring svc/<prometheus-service-name-from-above> 9090:9090
```
Then open `http://localhost:9090`.

Kind's control-plane components (etcd, scheduler, controller-manager, kube-proxy) are deliberately excluded from scraping — see the comment in `k8s/observability/kube-prometheus-stack-values.yaml` for why.

## Logging (Phase 4b)

Loki (log storage, monolithic mode, filesystem storage) + Grafana Alloy (log-shipping DaemonSet — Promtail's replacement, since Promtail hit end-of-life in March 2026) collect logs from every pod in the cluster automatically.

**Install:**
```bash
make logging-install
```

**View logs:** open Grafana, go to **Explore**, pick the **Loki** datasource, and query e.g.:
```
{namespace="microservices"}
```
or narrow to one service:
```
{namespace="microservices", pod=~"order-service.*"}
```

**Tear down when not actively using it** (this is the heaviest addition yet — see the resource note below):
```bash
make logging-uninstall
```

### A note on resource usage

Between Prometheus, Grafana, Alertmanager, Loki, and Alloy, the full observability stack is genuinely heavy for an 8GB machine running everything through WSL2. If you're not actively looking at dashboards or logs, tearing both down between sessions keeps the cluster lean and avoids Docker Desktop instability:
```bash
make logging-uninstall
make observability-uninstall
```
Both come back with one command each (`make observability-install`, `make logging-install`) whenever you want them again.

## CI/CD

Every push and pull request to `main` triggers `.github/workflows/ci.yml`, which:

1. Builds all three service images
2. Scans each with Trivy (report-only for now — see the workflow file for how to make it enforcing)
3. Spins up a throwaway Kind cluster inside the CI runner itself
4. Deploys the shared infrastructure and all three Helm charts into it
5. Runs `scripts/integration-test.sh` — the same register → seed product → create order → confirm stock decrement flow this project has been manually tested with all along, now automated
6. On pushes to `main` only (never on PRs), tags and pushes images to GHCR as `ghcr.io/<your-username>/microservices-platform-<service>:latest` and `:<commit-sha>`

Check results under your repo's **Actions** tab on GitHub.

A `Jenkinsfile` is also included, covering the same stages in Jenkins' declarative pipeline syntax — this project doesn't actually run Jenkins anywhere, it's there as a reference/portfolio artifact per the original spec.

## Project layout

```
microservices-platform/
├── docker-compose.yml
├── Makefile
├── .env.example
├── infra/
│   └── kind-config.yaml
└── services/
    ├── user-service/       (Node.js + Express + MongoDB)
    ├── order-service/      (Python + FastAPI + PostgreSQL)
    └── product-service/    (Go + Gin + Redis)
```
