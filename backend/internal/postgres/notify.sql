-- name: NotifyEvents :exec
SELECT pg_notify('gate_events', sqlc.arg(payload)::text);

-- name: ListenEvents :exec
LISTEN gate_events;

-- name: WakeJudgeWorkers :exec
SELECT pg_notify('judge_jobs', '');

-- name: ListenJudgeJobs :exec
LISTEN judge_jobs;
