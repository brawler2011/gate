# Strict judging

`judge.mode: strict` judges only on cores that give repeatable timing. Gate checks the host
(`backend/pkg/hostcheck`) but never changes it; this guide is the setup those checks expect.
A cloud VM will not pass: use bare metal, or a laptop on AC power.

Until every check passes, the worker takes no judging or build jobs, reports not ready on
`/ready`, and logs each failed check. Jobs wait in the queues. The checks run again every hour,
and any failure pauses judging until the worker restarts.

## Kernel

Pick the judge cores, for example `2-7`, and keep at least one core for everything else.
Kernel command line:

```
nosmt isolcpus=managed_irq,domain,2-7 nohz_full=2-7 rcu_nocbs=2-7 irqaffinity=0-1
```

After a reboot, `lscpu -e` lists the online CPUs, and `/sys/devices/system/cpu/isolated` and
`/sys/devices/system/cpu/nohz_full` show the judge cores. All judge cores must be in one NUMA node.

## Frequency

Turn turbo off and fix the frequency of every judge core. Neither setting survives a reboot, so
apply them from a boot-time unit:

```sh
echo 0 > /sys/devices/system/cpu/cpufreq/boost           # AMD and acpi-cpufreq
echo 1 > /sys/devices/system/cpu/intel_pstate/no_turbo   # intel_pstate
for c in 2 3 4 5 6 7; do
  d=/sys/devices/system/cpu/cpu$c/cpufreq
  echo performance > $d/scaling_governor
  echo 2000000 > $d/scaling_max_freq   # a frequency the cooling sustains, in kHz
  echo 2000000 > $d/scaling_min_freq
done
```

## Interrupts

`irqaffinity` keeps new IRQs off the judge cores. For a device IRQ that still reaches one (the
check names it), write the other cores to `/proc/irq/<n>/smp_affinity_list` and stop
`irqbalance`.

## k3s

The go-judge container must own the judge cores. Start k3s with the static CPU manager and
reserve the other cores:

```
--kubelet-arg=cpu-manager-policy=static --kubelet-arg=reserved-system-cpus=0-1
```

Changing the policy requires stopping k3s and deleting the kubelet's `cpu_manager_state` file.
The kubelet then gives a Guaranteed pod's integer CPU requests exclusive cores. Reserving every
core except the judge cores makes those the ones it gives.

## Gate

Config:

```yaml
go_judge:
  parallelism: 0
judge:
  mode: strict
  cpus: 2-7
```

Chart values: every container in the worker pod needs equal requests and limits, so the pod is Guaranteed.
go-judge requests exactly the judge cores:

```yaml
worker:
  strategy: Recreate
  resources:
    requests: {cpu: 500m, memory: 2Gi}
    limits: {cpu: 500m, memory: 2Gi}
goJudge:
  resources:
    requests: {cpu: 6, memory: 4Gi}
    limits: {cpu: 6, memory: 4Gi}
```
