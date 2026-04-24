# Falco — Runtime Security for GoMon

## Overview

Inspektor Gadget is an on-demand diagnostic tool — you run a gadget, observe, stop it. It is not designed for permanent enforcement or alerting.

Falco fills that gap. It is a permanent runtime security engine that sits in the kernel via an eBPF driver and continuously monitors system calls across all pods. When a call matches a rule — unexpected process execution, privilege escalation, namespace modification, unexpected outbound connection — Falco fires an alert.

In GoMon's context Falco would answer: *"is anything happening inside my pods right now that should not be happening?"*

---

## Architecture

Falco has three main components:

**1. Driver (eBPF or kernel module)**
Loaded into the kernel at startup as a DaemonSet. Captures system calls in real time using eBPF kprobes. Unlike Inspektor Gadget, the driver is permanent — it does not unload when a terminal session ends. On OpenShift this requires a privileged SCC bound to the Falco ServiceAccount.

**2. Falco Engine (Rules)**
Evaluates every captured syscall event against a ruleset. Rules are written in YAML and express conditions like:
- A shell was spawned inside a container
- A process wrote to `/etc` inside a container
- A container ran with a new privilege not in its baseline
- An outbound connection was made to an unexpected IP

Example rule relevant to GoMon:
```yaml
- rule: Unexpected process in agent pod
  desc: Detects any process execution in the agent container other than the agent binary
  condition: >
    spawned_process and
    k8s.pod.name startswith "agent" and
    not proc.name in (agent)
  output: >
    Unexpected process in agent pod
    (pod=%k8s.pod.name process=%proc.name user=%user.name)
  priority: WARNING
```

**3. Falco Sidekick (Alert Routing)**
Falco emits events as JSON. Falco Sidekick is a companion service that routes those events to downstream systems. In GoMon this would route to:
- **Elasticsearch** → alerts indexed and searchable in Kibana
- **Slack** → same Slack webhook already used by the alerting service
- **Alertmanager / VictoriaMetrics** → metrics on rule violation frequency

---

## What Falco Would Monitor in GoMon

| Rule | Pods | Why |
|---|---|---|
| Unexpected process execution | agent, aggregator, alerting | Only the Go binary should ever run |
| Shell spawned in container | all | `sh` or `bash` exec inside any pod is an incident |
| Outbound connection to unexpected IP | agent, aggregator | Agent should only connect to Kafka and Jaeger; aggregator to VictoriaMetrics |
| Privilege escalation | all | No GoMon pod requires elevated privileges at runtime |
| Write to sensitive path | all | No pod should write outside its designated volume mounts |

---

## OpenShift Deployment Considerations

Docker Desktop does not support Falco — its eBPF driver requires direct access to the host Linux kernel, which is not available through Docker Desktop's hypervisor VM.

On OpenShift (production deployment at Citi or any real cluster):

1. **SCC** — Falco requires a privileged SCC. Create a dedicated SCC rather than using the built-in `privileged` SCC to follow least-privilege:
```yaml
allowHostPID: true
allowHostNetwork: true
allowPrivilegedContainer: false
allowedCapabilities:
  - BPF
  - SYS_PTRACE
  - PERFMON
```

2. **ServiceAccount** — bind the SCC only to Falco's ServiceAccount, not cluster-wide.

3. **Operator** — deploy via the Falco Helm chart or the community OpenShift operator. The operator manages driver loading across nodes automatically on upgrades.

---

## Event Flow into GoMon's Observability Stack

```
Kernel syscall
    → Falco eBPF driver (DaemonSet, permanent)
        → Falco engine evaluates rule
            → Falco Sidekick
                ├── Logstash → Elasticsearch → Kibana
                └── Slack webhook (existing alerting integration)
```

This means Falco alerts appear in the same Kibana instance already used for GoMon application logs — no separate observability stack required.

---

## References

- [Falco Official Docs](https://falco.org/docs/)
- [Falco Rules Reference](https://falco.org/docs/rules/)
- [Falco Sidekick](https://github.com/falcosecurity/falcosidekick)
- [Falco on OpenShift](https://falco.org/docs/install-operate/deployment/openshift/)