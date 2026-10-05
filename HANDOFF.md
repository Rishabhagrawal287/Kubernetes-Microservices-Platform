# Kubernetes Microservices Platform — Handoff Summary
_Last updated: Oct 2026, after closing the CI/GitOps loop for the first time_

## Project Overview

Local, zero-cost, production-shaped Kubernetes microservices platform on WSL2 + Kind, ~8GB RAM host.

Stack: Node/Express/MongoDB (user-service), Python/FastAPI/PostgreSQL (order-service), Go/Gin/Redis (product-service), RabbitMQ, Argo CD (GitOps), Argo Rollouts (Blue/Green + Canary), Prometheus+Grafana, Loki+Alloy, Ollama+log-analyzer (AI log analysis), GitHub Actions CI.

Repo: https://github.com/Rishabhagrawal287/Kubernetes-Microservices-Platform (main branch)
Local path: ~/projects/microservices-platform
Cluster name: microservices-platform (context: kind-microservices-platform)

## Critical Architecture Fact

Cluster is SINGLE-NODE (control-plane only). Do NOT assume multi-node.

## 🎉 Major Milestone This Session: CI Fixed After 28 Broken Runs

CI had been broken since run #2 (silently, since nobody noticed — all deployment was manual).
**CI #29 was the first fully green run in project history.** The GitOps loop (build → scan → push to GHCR → auto-bump Helm values.yaml → commit back to main → Argo CD sync) is now proven working end to end, live.

### Root causes found and fixed, in order:
1. **Missing CRDs in CI's ephemeral Kind cluster.** Helm charts render `Rollout` (Argo Rollouts) and `ServiceMonitor` (Prometheus Operator) objects. CI's throwaway cluster never had those CRDs registered, so `helm install` failed with "no matches for kind." This has been broken since Phase 4a (ServiceMonitor) / Phase 5b (Rollout) — i.e. since CI run #2.
   - Fix: added a step installing both CRDs before `Deploy shared infrastructure`.
2. **CI's "wait for services" step checked `Deployment` objects, but services are `Rollout` objects** (converted in Phase 5b). Also, CI only had the Rollout *CRD* installed, not the actual *controller* — so `Rollout` objects were created but never reconciled into real pods.
   - Fix: installed the real Argo Rollouts controller (not just CRD), and changed the wait step to check `pod -l app=<service>` instead of `deployment/<service>`.
3. **`kubectl apply` on the Argo Rollouts install manifest failed: "annotation too large" (262144 byte cap).** Same root cause as the known Argo CD install gotcha (see below).
   - Fix: use `kubectl apply --server-side` for CRD/controller installs, not plain `apply`.
4. **Typo**: `github.com/argoprj/...` should be `github.com/argoproj/...` in the install URL. Caught before it caused a failure.
5. **Integration test script hardcoded product-service's old pre-fix port.** `scripts/integration-test.sh` line 19 port-forwarded `3003:8080`, but product-service's actual container port is `9091` (fixed in source back in CI #10, Aug 13 — the fix never made it into a pushed image because CI was broken, so this mismatch stayed invisible until the integration test could finally run).
   - Fix: changed port-forward to `3003:9091`.

### ci.yml current structure (as of this update)
Steps in order: Checkout → Build images → Trivy scan (report-only) → Create Kind cluster → Load images → **Install CRDs (Rollouts + ServiceMonitor)** → **Install Argo Rollouts controller** → Deploy shared infra → Wait for infra → Install services via Helm (dev values) → Wait for services (pod-label based) → Run integration test → (on push to main only:) push to GHCR → auto-bump Helm values.yaml tags → commit+push back with `[skip ci]`.

**Known gap, not yet fixed:** `log-analyzer` is built/scanned/pushed by CI but never actually installed via Helm or tested in the ephemeral cluster — only `user-service`, `order-service`, `product-service` are. Low priority since log-analyzer works fine on the live cluster; worth fixing if extending CI further.

## All 11 Original Roadmap Phases — STATUS: ALL COMPLETE ✅
(unchanged from original — see Environment Setup through Phase 7, all done and load-tested)

## Current Live Cluster State (as of this update)

- All 4 services running on fresh GHCR images, verified SHA tag `15dc1e148779ee7f0990394dbd59b7ea92af0331` (will be stale by the time you read this — check `kubectl get pods -n microservices -o jsonpath=...` per the health-check commands below)
- Argo CD manages 5 Applications, all Synced; user-service Blue/Green was manually promoted this session (old ReplicaSet scaled down and removed)
- Everything else (Ollama model, NetworkPolicies, backup CronJobs, HPA) unchanged from original setup — see original gotchas below, all still valid

## Key Gotchas / Lessons Learned (original + new this session)

**Original (still valid):**
- Loki chart 7.3.0: `deploymentMode: Monolithic` → `SingleBinary`. Already fixed, committed.
- NetworkPolicy egress: a policy with `policyTypes: [Egress]` and `podSelector: {}` restricts ALL pods' egress. Current fix (`allow-dns-and-intra-namespace-egress`) allows DNS everywhere + intra-namespace/monitoring/ai egress.
- NetworkPolicy + backup Jobs: Job pods don't inherit app labels; CronJobs patched with `backup-job: "true"` label.
- Ollama CPU throttling masquerades as random timeouts. Check `cpu.stat` for `nr_throttled`. Fixed by raising CPU limit 1→2 cores.
- `kubectl run --rm -it` often swallows output in this environment — re-verify with separate `kubectl run` + `kubectl exec` rather than trusting empty/missing output.
- Heredocs sometimes display garbled in terminal paste-back but the file usually still writes correctly — verify with `cat`/`grep`, not the terminal echo.
- `docker build` "cannot allocate memory" under memory pressure — usually fixed by `docker image prune -a -f` or retrying.
- **Argo CD / Argo Rollouts CRD installs: `kubectl apply -f <install.yaml>` hits "annotation too large" on big CRDs (262144 byte cap on `last-applied-configuration`). ALWAYS use `kubectl apply --server-side --force-conflicts` (or just `--server-side`) for these.** This bit us twice now — once for Argo CD's own install originally, once for Argo Rollouts' CRD in CI this session.
- 8GB RAM is a hard constraint — use `scripts/mode-ai.sh` / `scripts/mode-observability.sh` to scale services when testing one subsystem heavily.

**New this session:**
- **CI's ephemeral Kind cluster needs its OWN copies of any CRDs/controllers your Helm charts depend on** (Argo Rollouts, Prometheus Operator, etc.) — these aren't automatically available just because they're installed on your real cluster. If you add a new CRD-dependent resource type to any chart in the future, CI will break the same way until you add the matching install step.
- **CI readiness checks must match the actual resource kind.** If a service is converted from `Deployment` to `Rollout` (or any other kind), update `kubectl wait` steps accordingly — `kubectl wait --for=condition=available deployment/X` silently doesn't exist for a `Rollout`.
- **user-service has a DNS-on-startup fragility**: if MongoDB can't be resolved at container start (e.g. during a network blip), the Node app logs `getaddrinfo EAI_AGAIN mongo` and never retries — pod stays "Running" but `0/1 Ready` forever, even after networking recovers. Happened twice this session, both after external network disruptions (not app bugs we introduced). **Workaround:** `kubectl delete pod -n microservices -l app=user-service` once networking is confirmed healthy — safe, pods recreate cleanly. **Real fix (not yet done):** add retry/backoff logic to user-service's MongoDB connection code.
- **WSL2/Docker Desktop networking can fail at the OS level**, independent of Docker or Kubernetes — symptoms: `ping 8.8.8.8` fails with "Destination Host Unreachable," `kubectl` commands hang with TLS handshake timeouts, Docker Desktop shows "Engine stopped unexpectedly" with a WSL command timeout error. **Fix: full Windows restart** (not just `wsl --shutdown`, which can leave things half-wedged). After restart: verify `ping 8.8.8.8` works before touching Docker/k8s, then bring the cluster back up per the health-check commands below.
- `kubectl get rollout` (plain kubectl) doesn't show a Status/pause column — use `kubectl argo rollouts get rollout <name> -n microservices` (the plugin) for full Blue/Green/Canary state including pause status.

## Quick Cluster Health Check Commands (run these first in any new session)

```bash
docker ps | grep microservices-platform
# if not "Up", run: docker start microservices-platform-control-plane
kind export kubeconfig --name microservices-platform
kubectl get nodes
kubectl get applications -n argocd
kubectl get pods -n microservices
kubectl get pods -n monitoring
kubectl get pods -n ai
```

If `kubectl` commands hang or time out: check `ping -c 2 8.8.8.8` first — if that fails too, it's WSL2/Docker Desktop networking, not your cluster. Full Windows restart fixes it (see gotcha above).

## Remaining Open Items (as of this update)

1. **[Optional, in progress]** Add retry/backoff logic to user-service's MongoDB connection to stop the DNS-on-startup fragility.
2. **[Optional, low priority]** Add `log-analyzer` to CI's `Install services via Helm` step so it's actually deploy-tested, not just built/scanned/pushed.
3. **[Optional, deprioritized, unchanged from original]** Traefik + Ingress — never set up, no ingress controller exists.
4. **[Optional, deprioritized, unchanged from original]** Distributed tracing (Tempo/Jaeger) — not started.

## User Preferences / Working Style Notes

- Wants numbered/lettered step-by-step instructions, one block at a time when things are risky, batched when safe
- Prefers to paste raw terminal output back rather than summarizing it themselves
- Appreciates being told explicitly when something is reversible vs. risky before running it
- When presenting options/choices, wants a clear explanation of what each option IS, how it's used, and why one might be preferred — not just a bare list
- Working in WSL2 Ubuntu terminal; occasional terminal display glitches (garbled heredoc paste-back, swallowed `kubectl run --rm -it` output) are environment quirks, not real errors — worth re-verifying before treating as a bug
- Tends to step away from this project for extended periods (days to weeks) — keep this doc current after any significant session
