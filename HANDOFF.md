# Kubernetes Microservices Platform — Handoff Summary
_Last updated: Oct 2026, after the user-service duplicate-retry fix and the markdown paths-ignore change_

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

**Resolved (CI #32):** `log-analyzer` is now installed via Helm in CI using `helm/log-analyzer/values-dev.yaml` (which overrides the image to the locally built `log-analyzer:local`) and readiness-checked alongside the other three services. Its `/ready` endpoint is unconditional, so CI does not need Loki or Ollama. Caveat: CI does not exercise log-analyzer's `/analyze` path, only that it starts and becomes ready.

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
- **user-service MongoDB startup retry (fixed in code):** previously, if MongoDB could not be resolved at container start, the Node app logged `getaddrinfo EAI_AGAIN mongo` and never retried, leaving the pod Running but 0/1 Ready forever. `services/user-service/src/index.js` now retries every 5s. The first version (CI #31/#32 builds, promoted to stable) was seen recovering from a real `EAI_AGAIN` on the live preview pods. It also had a bug: a failed connect fired both the `.catch()` handler and the `disconnected` event, scheduling two overlapping retries, and the app logged a false `connected to MongoDB` after every failed attempt against an unreachable host (reproduced locally with a nonexistent host; `/ready` stayed correct because it also checks `mongoose.connection.readyState === 1`; exact mechanism not verified). Fixed with a single-pending-timer guard, `scheduleRetry()`, and verified side by side in Docker (old image: false `connected` line every cycle; new image: none). Fallback if a pod is ever wedged: `kubectl delete pod -n microservices -l app=user-service`.
- **Every push to `main` triggers a full CI run, a bot tag-bump commit, and (for user-service) a new Blue/Green preview that waits for a manual promote.** This includes docs-only pushes (found the hard way with HANDOFF.md). `ci.yml` now has `paths-ignore: ['**.md']` so markdown-only pushes skip CI (confirmed: the markdown-only push of the rebuild section started no CI run). After a code-changing push: wait for CI and the bot commit, run `git pull --rebase origin main`, then `kubectl argo rollouts get rollout user-service -n microservices`, and when a healthy preview shows, `kubectl argo rollouts promote user-service -n microservices` (rollback: `kubectl argo rollouts undo user-service -n microservices`). Check which revision is current before promoting, since a newer CI run can replace the preview.
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

**Completed since the original doc:**
- user-service MongoDB retry/backoff, including the duplicate-retry fix
- log-analyzer added to CI Helm install and readiness check (CI #32)
- Stray `master` branch on the remote (single July 16 commit) deleted after confirming no Argo CD app tracked it; its files were not fully diffed against main. Recoverable from a local clone while the commit survives: `git push origin bb5d00a:refs/heads/master`

**Still open (all optional):**
1. Traefik + Ingress: never set up, no ingress controller exists. The Kind node already maps host ports 80/443, so it is partly ready.
2. Distributed tracing (Tempo/Jaeger): not started; needs SDK instrumentation in all 3 language runtimes plus extra RAM.

## User Preferences / Working Style Notes

- Wants numbered/lettered step-by-step instructions, one block at a time when things are risky, batched when safe
- Prefers to paste raw terminal output back rather than summarizing it themselves
- Appreciates being told explicitly when something is reversible vs. risky before running it
- When presenting options/choices, wants a clear explanation of what each option IS, how it's used, and why one might be preferred — not just a bare list
- Working in WSL2 Ubuntu terminal; occasional terminal display glitches (garbled heredoc paste-back, swallowed `kubectl run --rm -it` output) are environment quirks, not real errors — worth re-verifying before treating as a bug
- Tends to step away from this project for extended periods (days to weeks) — keep this doc current after any significant session

## Rebuilding From Scratch (cluster lost, deleted, or new machine)

_Written from the repo contents (Makefile, k8s/, infra/, ci.yml) and what we learned debugging. A full rebuild has NOT been done from this doc: treat it as a map, not a tested procedure. **[repo]** = taken from a file in this repo. **[inferred]** = reasoned, not run. **[GAP]** = cannot be done from the repo alone yet._

**What is lost with the cluster:** contents of mongo, postgres and redis, and the `db-backups` PVC (the backup CronJobs write inside the cluster). GitHub holds code and config only.

**Traps in the existing Makefile (written before Phases 5-7):**
- Do NOT use `make kind-up`. It uses `infra/kind-config.yaml`, the old 3-node config that hit the CNI bug. Use `infra/kind-config-singlenode.yaml`.
- `build-images`, `kind-load` and `helm-install` cover only 3 services with local images (`values-dev.yaml`). log-analyzer is missing, and the live cluster now runs GHCR images deployed by Argo CD, not Helm by hand.
- The Makefile stops at logging (Phase 4b). Argo CD, Argo Rollouts, Ollama, network policies and backups have no Makefile targets.

**Order:**
1. Prerequisites: Docker, kind, kubectl, helm, and the `kubectl-argo-rollouts` plugin. [inferred]
2. Cluster: `kind create cluster --config infra/kind-config-singlenode.yaml` [repo]. Check that the cluster name inside the file is `microservices-platform` and that it maps host ports 80/443 (the running node did). [inferred]
3. Namespaces and shared infra: `make k8s-infra` (namespace, secrets, mongo, postgres, redis, rabbitmq). [repo]
4. Argo Rollouts, same commands CI uses [repo: ci.yml]:
   - `kubectl create namespace argo-rollouts`
   - `kubectl apply --server-side -n argo-rollouts -f https://github.com/argoproj/argo-rollouts/releases/latest/download/install.yaml`
5. Monitoring BEFORE the app charts: `make observability-install`, then `make logging-install`. [repo: Makefile] Reason: the charts render `ServiceMonitor` objects, and that CRD most likely comes from kube-prometheus-stack. Without it the charts fail the way CI did. [inferred from the CI failure]
6. Argo CD: `kubectl create namespace argocd`, then `kubectl apply --server-side --force-conflicts -n argocd -f <install.yaml>` (plain `apply` hits the annotation-too-large error). **[GAP]** The install manifest URL and pinned version are not recorded in the repo.
7. Argo CD Applications: five apps (user-service, order-service, product-service, log-analyzer, infra), all tracking `main`. **[GAP]** Their manifests are not in the repo, and the `infra` app's source path is not recorded.
8. Ollama in namespace `ai`, model `qwen2.5:1.5b`, CPU limit 2 cores. **[GAP]** No manifests in the repo. After it starts, the model probably has to be pulled again. [inferred]
9. NetworkPolicies, only after the services are healthy: `kubectl apply -f k8s/network-policies/microservices-policies.yaml` [repo]. A bad egress policy silently blocks all outbound traffic (see Gotchas above).
10. Backups: `kubectl apply -f k8s/backups/db-backups-pvc.yaml`, then `kubectl apply -f k8s/backups/db-backup-cronjobs.yaml` [repo]. Check that the CronJob pod templates carry the `backup-job: "true"` label (see Gotchas above). Whether the committed YAML has it is unverified.
11. Verify with the Quick Cluster Health Check commands above.

**Image pulls:** the Helm values point at `ghcr.io/rishabhagrawal287/...` tagged with a commit SHA. The live cluster pulled new tags without any pull secret that we know of, so the packages are probably public. If pods show `ImagePullBackOff` with 401/403, check the package visibility on GitHub. The tagged images must also still exist in GHCR. [inferred]
