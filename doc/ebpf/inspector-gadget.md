# Inspektor Gadget — eBPF Diagnostics for GoMon

## Overview

Inspektor Gadget is an on-demand eBPF diagnostic tool for Kubernetes. It deploys a DaemonSet that acts as a loader/proxy — when a `kubectl gadget run` command is issued, the DaemonSet loads the requested eBPF program into the kernel, streams events to the terminal, and unloads the program on exit.

It is not a permanent monitoring solution. It is the production equivalent of `strace` or `tcpdump` — used during incidents and debugging sessions, not left running continuously.

---

## Installation

```bash
kubectl krew install gadget
kubectl gadget deploy
```

Requires Linux kernel 5.10+ with BTF enabled. Verified working on Docker Desktop with kernel 6.12.5-linuxkit.

**Limitation on Docker Desktop:** Gadgets requiring the SocketEnricher operator (`trace_dns`, `advise_networkpolicy`) fail because Docker Desktop's Linux VM does not run systemd. These gadgets work correctly on real Linux nodes (OpenShift, EKS, GKE).

---

## Gadgets Used in GoMon

### trace_tcp
Traces TCP connection open/close events per pod using eBPF kprobes.

```bash
kubectl gadget run ghcr.io/inspektor-gadget/gadget/trace_tcp:latest -n monitoring
# Filter to specific services:
kubectl gadget run ghcr.io/inspektor-gadget/gadget/trace_tcp:latest -n monitoring | grep -i 'agent\|aggregator'
```

**Output fields:**

| Field | Meaning |
|---|---|
| `K8S.PODNAME` | Pod generating the event |
| `COMM` | Process name inside the container |
| `SRC / DST` | Source and destination addresses. `p/` prefix = resolved to pod name. `r/` prefix = raw IP |
| `TYPE` | `connect` (outbound), `accept` (inbound), `close` |

**Findings in GoMon:**
- Agent → Kafka-1:9092 — persistent `ESTABLISHED` connection (verified via `/proc/net/tcp`)
- Agent → Jaeger:14268 — persistent `ESTABLISHED` connection for trace shipping
- elasticsearch-lb (nginx) → external ES VMs — short-lived connections per request, indicating no connection pooling
- Aggregator inbound on :2113 — healthcheck/scrape from VictoriaMetrics

**Verifying persistent connections without trace_tcp:**
```bash
kubectl exec -it agent-wdln4 -n monitoring -- cat /proc/net/tcp
```
Decoding hex addresses: IP is little-endian, port is hex. `ESTABLISHED` = state `01`, `LISTEN` = state `0A`.

---

### trace_exec
Traces every `execve()` syscall inside pods — captures any process execution at kernel level.

```bash
kubectl gadget run ghcr.io/inspektor-gadget/gadget/trace_exec:latest -n monitoring
```

**Why `execve()` is the right hook:** It is the single syscall that replaces a process image with a new binary. Every program launch on Linux must call it — there is no bypass. Hooking it guarantees capture of the binary path, arguments, parent process, and container, before the program executes a single instruction.

**Findings in GoMon:**
- PostgreSQL liveness probe fires `pg_isready` every ~2 seconds via the `fork()` → `execve()` pattern — kubelet forks a child, child replaces itself with `pg_isready`
- `ls` executed via `kubectl exec` captured immediately, attributed to the correct pod and container

**Production use case:** Define an allowed execution baseline per pod. Any `execve` outside that baseline is an incident. This baseline is what Falco rules encode for permanent enforcement.

---

### profile_cpu
Samples kernel stack traces per pod to identify CPU bottlenecks — no application instrumentation required.

```bash
# All pods in namespace
kubectl gadget run ghcr.io/inspektor-gadget/gadget/profile_cpu:latest -n monitoring --timeout 10

# Specific pod
kubectl gadget run ghcr.io/inspektor-gadget/gadget/profile_cpu:latest -n monitoring -p <pod-name> --timeout 10

# Specific container within pod
kubectl gadget run ghcr.io/inspektor-gadget/gadget/profile_cpu:latest -n monitoring -p <pod-name> -c <container-name> --timeout 10
```

**Key fields:**

| Field | Meaning |
|---|---|
| `SAMPLES` | How many CPU samples caught — higher = more CPU time |
| `KERN_STACK` | Kernel call stack at moment of sample — highest frequency stack is the bottleneck |

**Findings in GoMon:**
- Aggregator showed 0 samples when Kafka topic `metrics-v4` did not exist (idle, no work)
- After topic recreation and aggregator restart, samples appeared confirming active consumption
- Low sample count confirms aggregator is I/O bound (Kafka read → VictoriaMetrics write), not CPU bound

**Note:** Full kernel stacks require a real Linux node. Docker Desktop on ARM64 produces `stack lost` warnings due to eBPF map timing constraints.

---

### trace_oomkill
Passively hooks the kernel OOM killer. Zero overhead until an OOM event occurs, then captures the killing process, victim process, and memory pages consumed.

```bash
kubectl gadget run ghcr.io/inspektor-gadget/gadget/trace_oomkill:latest -n monitoring
```

**Key fields:**

| Field | Meaning |
|---|---|
| `COMM` | Process that triggered memory pressure |
| `TCOMM` | Process the kernel chose to kill (victim) |
| `TPID` | Victim PID |
| `PAGES` | Memory pages victim was using (pages × 4KB = bytes) |

**Production use case:** `kubectl describe pod` only shows `OOMKilled` after the fact. `trace_oomkill` captures the event at the kernel level before kubelet is aware — identifying which process caused pressure, not just which was killed.

---

## Persistent Gadget Execution

`kubectl gadget run` ties the eBPF program lifecycle to the terminal session. For persistent execution two options exist:

1. **GadgetTrace CRD** — define the gadget as a Kubernetes object. The Inspektor Gadget operator keeps it loaded independent of terminal sessions.
2. **Falco** — the production answer for permanent kernel-level monitoring with rule-based alerting. See `falco.md`.

---

## Incident Discovered During eBPF Session

While profiling the aggregator, `trace_exec` and `profile_cpu` revealed the aggregator was idle. Investigation showed:

- Kafka topic `metrics-v4` had been lost after broker restarts
- Aggregator was logging `[15] Group Coordinator Not Available`
- Agent was logging `[3] Unknown Topic Or Partition`

**Resolution:**
```bash
# Recreate topic
kubectl exec -it kafka-0 -n monitoring -- kafka-topics.sh \
  --bootstrap-server localhost:9092 \
  --create \
  --topic metrics-v4 \
  --partitions 3 \
  --replication-factor 3

# Restart aggregator to rejoin consumer group
kubectl rollout restart deployment/aggregator -n monitoring

# Verify lag = 0 across all partitions
kubectl exec -it kafka-0 -n monitoring -- kafka-consumer-groups.sh \
  --bootstrap-server localhost:9092 \
  --describe \
  --all-groups
```

**Root cause:** Kafka topic configuration is not persisted across broker restarts in this setup. Production fix: use a Kafka operator (Strimzi) or ensure topic configuration is applied via init job in ArgoCD on every deployment.

---

## References

- [Inspektor Gadget Docs](https://inspektor-gadget.io/docs/latest/)
- [All Available Gadgets](https://inspektor-gadget.io/docs/latest/gadgets/)
- [Install on Kubernetes](https://inspektor-gadget.io/docs/latest/getting-started/install-kubernetes/)