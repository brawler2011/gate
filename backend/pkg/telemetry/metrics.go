package telemetry

import (
	"context"
	"math"
	"os"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	processStartTime = time.Now()

	// Outbox atomic state
	outboxPendingCount atomic.Int64
	outboxDispatchLag  atomic.Uint64 // stored as math.Float64bits

	// Sandbox atomic state
	sandboxActiveExecutions atomic.Int64

	// Judge atomic state
	judgeActiveWorkers atomic.Int64

	// Instruments
	outboxDispatchedCounter metric.Int64Counter
	outboxFailedCounter     metric.Int64Counter

	natsAckCounter metric.Int64Counter
	natsNakCounter metric.Int64Counter

	gojudgeCallDuration metric.Float64Histogram
	gojudgeMemoryBytes  metric.Int64Histogram
	gojudgeTimeMs       metric.Int64Histogram

	judgeSubmissionsCounter metric.Int64Counter
	judgeDurationHistogram  metric.Float64Histogram
	judgeQueueWaitHistogram metric.Float64Histogram
	judgeRetriesCounter     metric.Int64Counter
)

func initCustomMetrics(mp metric.MeterProvider) error {
	meter := mp.Meter("gate-telemetry")

	var err error

	// Outbox counters
	outboxDispatchedCounter, err = meter.Int64Counter("outbox_dispatched_events_total",
		metric.WithDescription("Total outbox events dispatched"))
	if err != nil {
		return err
	}

	outboxFailedCounter, err = meter.Int64Counter("outbox_events_failed_total",
		metric.WithDescription("Total outbox events failed"))
	if err != nil {
		return err
	}

	// NATS counters
	natsAckCounter, err = meter.Int64Counter("nats_consumer_ack_total",
		metric.WithDescription("Total NATS messages acknowledged"))
	if err != nil {
		return err
	}

	natsNakCounter, err = meter.Int64Counter("nats_consumer_nak_total",
		metric.WithDescription("Total NATS messages not acknowledged / negatively acknowledged"))
	if err != nil {
		return err
	}

	// GoJudge histograms
	gojudgeCallDuration, err = meter.Float64Histogram("gojudge_grpc_call_duration_seconds",
		metric.WithDescription("Duration of gRPC calls to go-judge sandbox in seconds"))
	if err != nil {
		return err
	}

	gojudgeMemoryBytes, err = meter.Int64Histogram("gojudge_sandbox_memory_bytes",
		metric.WithDescription("Memory consumed in go-judge sandbox in bytes"))
	if err != nil {
		return err
	}

	gojudgeTimeMs, err = meter.Int64Histogram("gojudge_sandbox_time_ms",
		metric.WithDescription("Execution time in go-judge sandbox in milliseconds"))
	if err != nil {
		return err
	}

	// Judge counters and histograms
	judgeSubmissionsCounter, err = meter.Int64Counter("judge_submissions_total",
		metric.WithDescription("Total submissions processed by judge"))
	if err != nil {
		return err
	}

	judgeDurationHistogram, err = meter.Float64Histogram("judge_duration_seconds",
		metric.WithDescription("Duration of judging submissions in seconds"))
	if err != nil {
		return err
	}

	judgeQueueWaitHistogram, err = meter.Float64Histogram("judge_queue_wait_seconds",
		metric.WithDescription("Time submission spent waiting in queue before judging in seconds"))
	if err != nil {
		return err
	}

	judgeRetriesCounter, err = meter.Int64Counter("judge_retries_total",
		metric.WithDescription("Total retried judging attempts"))
	if err != nil {
		return err
	}

	// Observable gauges for system, outbox, sandbox, judge
	var (
		startTimeGauge   metric.Float64ObservableGauge
		cpuSecondsGauge  metric.Float64ObservableGauge
		openFDsGauge     metric.Int64ObservableGauge
		maxFDsGauge      metric.Int64ObservableGauge
		goroutinesGauge  metric.Int64ObservableGauge
		threadsGauge     metric.Int64ObservableGauge
		memAllocGauge    metric.Int64ObservableGauge
		memSysGauge      metric.Int64ObservableGauge
		memHeapInuse     metric.Int64ObservableGauge
		memHeapIdle      metric.Int64ObservableGauge
		memStackInuse    metric.Int64ObservableGauge
		memMallocsGauge  metric.Int64ObservableGauge
		memFreesGauge    metric.Int64ObservableGauge
		gcCountGauge     metric.Int64ObservableGauge
		gcSumGauge       metric.Float64ObservableGauge

		outboxPendingGauge metric.Int64ObservableGauge
		outboxLagGauge     metric.Float64ObservableGauge

		gojudgeActiveGauge metric.Int64ObservableGauge
		judgeWorkersGauge  metric.Int64ObservableGauge
	)

	startTimeGauge, err = meter.Float64ObservableGauge("process_start_time_seconds",
		metric.WithDescription("Process start time in unix seconds"))
	if err != nil {
		return err
	}
	cpuSecondsGauge, err = meter.Float64ObservableGauge("process_cpu_seconds_total",
		metric.WithDescription("Total user and system CPU time spent in seconds"))
	if err != nil {
		return err
	}
	openFDsGauge, err = meter.Int64ObservableGauge("process_open_fds",
		metric.WithDescription("Number of open file descriptors"))
	if err != nil {
		return err
	}
	maxFDsGauge, err = meter.Int64ObservableGauge("process_max_fds",
		metric.WithDescription("Maximum open file descriptors limit"))
	if err != nil {
		return err
	}
	goroutinesGauge, err = meter.Int64ObservableGauge("go_goroutines",
		metric.WithDescription("Number of goroutines that currently exist"))
	if err != nil {
		return err
	}
	threadsGauge, err = meter.Int64ObservableGauge("go_threads",
		metric.WithDescription("Number of OS threads created"))
	if err != nil {
		return err
	}
	memAllocGauge, err = meter.Int64ObservableGauge("go_memstats_alloc_bytes",
		metric.WithDescription("Number of bytes allocated and still in use"))
	if err != nil {
		return err
	}
	memSysGauge, err = meter.Int64ObservableGauge("go_memstats_sys_bytes",
		metric.WithDescription("Number of bytes obtained from system"))
	if err != nil {
		return err
	}
	memHeapInuse, err = meter.Int64ObservableGauge("go_memstats_heap_inuse_bytes",
		metric.WithDescription("Number of heap bytes in use"))
	if err != nil {
		return err
	}
	memHeapIdle, err = meter.Int64ObservableGauge("go_memstats_heap_idle_bytes",
		metric.WithDescription("Number of heap bytes idle"))
	if err != nil {
		return err
	}
	memStackInuse, err = meter.Int64ObservableGauge("go_memstats_stack_inuse_bytes",
		metric.WithDescription("Number of stack bytes in use"))
	if err != nil {
		return err
	}
	memMallocsGauge, err = meter.Int64ObservableGauge("go_memstats_mallocs_total",
		metric.WithDescription("Total number of mallocs"))
	if err != nil {
		return err
	}
	memFreesGauge, err = meter.Int64ObservableGauge("go_memstats_frees_total",
		metric.WithDescription("Total number of frees"))
	if err != nil {
		return err
	}
	gcCountGauge, err = meter.Int64ObservableGauge("go_gc_duration_seconds_count",
		metric.WithDescription("Number of GC cycles completed"))
	if err != nil {
		return err
	}
	gcSumGauge, err = meter.Float64ObservableGauge("go_gc_duration_seconds_sum",
		metric.WithDescription("Total GC pause duration in seconds"))
	if err != nil {
		return err
	}

	outboxPendingGauge, err = meter.Int64ObservableGauge("outbox_pending_events_count",
		metric.WithDescription("Pending events in outbox"))
	if err != nil {
		return err
	}
	outboxLagGauge, err = meter.Float64ObservableGauge("outbox_dispatch_lag_seconds",
		metric.WithDescription("Outbox dispatch age lag in seconds"))
	if err != nil {
		return err
	}

	gojudgeActiveGauge, err = meter.Int64ObservableGauge("gojudge_active_executions",
		metric.WithDescription("Active parallel executions in sandbox"))
	if err != nil {
		return err
	}
	judgeWorkersGauge, err = meter.Int64ObservableGauge("judge_active_workers",
		metric.WithDescription("Number of active judge workers"))
	if err != nil {
		return err
	}

	_, err = meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		// 1. Process stats
		o.ObserveFloat64(startTimeGauge, float64(processStartTime.Unix()))

		var rusage syscall.Rusage
		if err := syscall.Getrusage(syscall.RUSAGE_SELF, &rusage); err == nil {
			cpuSec := float64(rusage.Utime.Sec) + float64(rusage.Utime.Usec)/1e6 +
				float64(rusage.Stime.Sec) + float64(rusage.Stime.Usec)/1e6
			o.ObserveFloat64(cpuSecondsGauge, cpuSec)
		}

		if fds, err := os.ReadDir("/proc/self/fd"); err == nil {
			o.ObserveInt64(openFDsGauge, int64(len(fds)))
		}
		var rlimit syscall.Rlimit
		if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit); err == nil {
			o.ObserveInt64(maxFDsGauge, int64(rlimit.Cur))
		}

		// 2. Go runtime stats
		o.ObserveInt64(goroutinesGauge, int64(runtime.NumGoroutine()))
		o.ObserveInt64(threadsGauge, runtime.NumCgoCall())

		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		o.ObserveInt64(memAllocGauge, int64(m.Alloc))
		o.ObserveInt64(memSysGauge, int64(m.Sys))
		o.ObserveInt64(memHeapInuse, int64(m.HeapInuse))
		o.ObserveInt64(memHeapIdle, int64(m.HeapIdle))
		o.ObserveInt64(memStackInuse, int64(m.StackInuse))
		o.ObserveInt64(memMallocsGauge, int64(m.Mallocs))
		o.ObserveInt64(memFreesGauge, int64(m.Frees))
		o.ObserveInt64(gcCountGauge, int64(m.NumGC))
		o.ObserveFloat64(gcSumGauge, float64(m.PauseTotalNs)/1e9)

		// 3. Outbox stats
		o.ObserveInt64(outboxPendingGauge, outboxPendingCount.Load())
		o.ObserveFloat64(outboxLagGauge, math.Float64frombits(outboxDispatchLag.Load()))

		// 4. Sandbox & Judge active stats
		o.ObserveInt64(gojudgeActiveGauge, sandboxActiveExecutions.Load())
		o.ObserveInt64(judgeWorkersGauge, judgeActiveWorkers.Load())

		return nil
	}, startTimeGauge, cpuSecondsGauge, openFDsGauge, maxFDsGauge, goroutinesGauge, threadsGauge,
		memAllocGauge, memSysGauge, memHeapInuse, memHeapIdle, memStackInuse, memMallocsGauge, memFreesGauge,
		gcCountGauge, gcSumGauge, outboxPendingGauge, outboxLagGauge, gojudgeActiveGauge, judgeWorkersGauge)

	return err
}

// RegisterPGXPoolMetrics registers PostgreSQL connection pool gauges.
func RegisterPGXPoolMetrics(pool *pgxpool.Pool) error {
	meter := otel.GetMeterProvider().Meter("gate-postgres")

	connsGauge, err := meter.Int64ObservableGauge("pgxpool_connections_total",
		metric.WithDescription("Number of connections in postgres pool"))
	if err != nil {
		return err
	}

	maxConnsGauge, err := meter.Int64ObservableGauge("pgxpool_max_conns",
		metric.WithDescription("Maximum number of connections allowed in postgres pool"))
	if err != nil {
		return err
	}

	acquireWaitGauge, err := meter.Int64ObservableGauge("pgxpool_acquire_wait_total",
		metric.WithDescription("Total number of times a connection was acquired"))
	if err != nil {
		return err
	}

	acquireWaitDurGauge, err := meter.Float64ObservableGauge("pgxpool_acquire_wait_duration_seconds_total",
		metric.WithDescription("Total duration spent waiting for connections in seconds"))
	if err != nil {
		return err
	}

	_, err = meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		stat := pool.Stat()
		o.ObserveInt64(connsGauge, int64(stat.AcquiredConns()), metric.WithAttributes(attribute.String("state", "acquired")))
		o.ObserveInt64(connsGauge, int64(stat.IdleConns()), metric.WithAttributes(attribute.String("state", "idle")))
		o.ObserveInt64(maxConnsGauge, int64(stat.MaxConns()))
		o.ObserveInt64(acquireWaitGauge, stat.AcquireCount())
		o.ObserveFloat64(acquireWaitDurGauge, stat.AcquireDuration().Seconds())
		return nil
	}, connsGauge, maxConnsGauge, acquireWaitGauge, acquireWaitDurGauge)

	return err
}

// RegisterNATSMetrics registers NATS JetStream stream and consumer gauges.
func RegisterNATSMetrics(js jetstream.JetStream) error {
	meter := otel.GetMeterProvider().Meter("gate-nats")

	streamMsgsGauge, err := meter.Int64ObservableGauge("nats_stream_messages",
		metric.WithDescription("Number of messages in NATS stream"))
	if err != nil {
		return err
	}

	streamBytesGauge, err := meter.Int64ObservableGauge("nats_stream_bytes",
		metric.WithDescription("Bytes in NATS stream"))
	if err != nil {
		return err
	}

	consumerPendingGauge, err := meter.Int64ObservableGauge("nats_consumer_num_pending",
		metric.WithDescription("Number of pending messages for consumer"))
	if err != nil {
		return err
	}

	_, err = meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		sInfo, err := js.Stream(ctx, "SUBMISSIONS")
		if err == nil && sInfo != nil {
			info, err := sInfo.Info(ctx)
			if err == nil && info != nil {
				o.ObserveInt64(streamMsgsGauge, int64(info.State.Msgs), metric.WithAttributes(attribute.String("stream", "SUBMISSIONS")))
				o.ObserveInt64(streamBytesGauge, int64(info.State.Bytes), metric.WithAttributes(attribute.String("stream", "SUBMISSIONS")))
			}
		}

		c, err := js.Consumer(ctx, "SUBMISSIONS", "judge_consumer")
		if err == nil && c != nil {
			info, err := c.Info(ctx)
			if err == nil && info != nil {
				o.ObserveInt64(consumerPendingGauge, int64(info.NumPending), metric.WithAttributes(attribute.String("consumer", "judge_consumer")))
			}
		}
		return nil
	}, streamMsgsGauge, streamBytesGauge, consumerPendingGauge)

	return err
}

// Outbox metric recording helpers
func RecordOutboxPending(count int64) {
	outboxPendingCount.Store(count)
}

func RecordOutboxLag(lagSeconds float64) {
	outboxDispatchLag.Store(math.Float64bits(lagSeconds))
}

func RecordOutboxDispatched(ctx context.Context, count int64) {
	if outboxDispatchedCounter != nil {
		outboxDispatchedCounter.Add(ctx, count)
	}
}

func RecordOutboxFailed(ctx context.Context, count int64) {
	if outboxFailedCounter != nil {
		outboxFailedCounter.Add(ctx, count)
	}
}

// NATS metric recording helpers
func RecordNATSAck(ctx context.Context) {
	if natsAckCounter != nil {
		natsAckCounter.Add(ctx, 1)
	}
}

func RecordNATSNak(ctx context.Context) {
	if natsNakCounter != nil {
		natsNakCounter.Add(ctx, 1)
	}
}

// Sandbox metric recording helpers
func IncSandboxActive() {
	sandboxActiveExecutions.Add(1)
}

func DecSandboxActive() {
	sandboxActiveExecutions.Add(-1)
}

func RecordSandboxExecution(ctx context.Context, language string, memoryBytes int64, timeMs int64, grpcDuration float64) {
	attrs := metric.WithAttributes(attribute.String("language", language))
	if gojudgeCallDuration != nil {
		gojudgeCallDuration.Record(ctx, grpcDuration)
	}
	if gojudgeMemoryBytes != nil {
		gojudgeMemoryBytes.Record(ctx, memoryBytes, attrs)
	}
	if gojudgeTimeMs != nil {
		gojudgeTimeMs.Record(ctx, timeMs, attrs)
	}
}

// Judge metric recording helpers
func SetJudgeActiveWorkers(count int64) {
	judgeActiveWorkers.Store(count)
}

func RecordJudgeSubmission(ctx context.Context, contestID, problemID, language, verdict string) {
	if judgeSubmissionsCounter != nil {
		judgeSubmissionsCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("contest_id", contestID),
			attribute.String("problem_id", problemID),
			attribute.String("language", language),
			attribute.String("verdict", verdict),
		))
	}
}

func RecordJudgeDuration(ctx context.Context, durationSeconds float64) {
	if judgeDurationHistogram != nil {
		judgeDurationHistogram.Record(ctx, durationSeconds)
	}
}

func RecordJudgeQueueWait(ctx context.Context, waitSeconds float64) {
	if judgeQueueWaitHistogram != nil {
		judgeQueueWaitHistogram.Record(ctx, waitSeconds)
	}
}

func RecordJudgeRetry(ctx context.Context) {
	if judgeRetriesCounter != nil {
		judgeRetriesCounter.Add(ctx, 1)
	}
}
