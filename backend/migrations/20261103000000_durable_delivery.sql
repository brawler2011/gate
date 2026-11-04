-- +goose Up
CREATE TABLE event_aggregates (
    aggregate_id uuid PRIMARY KEY,
    version bigint NOT NULL CHECK (version > 0)
);
ALTER TABLE outbox_events
    ADD COLUMN aggregate_version bigint NOT NULL,
    ADD COLUMN lease_token uuid,
    ADD COLUMN next_attempt_at timestamptz NOT NULL DEFAULT now();
CREATE UNIQUE INDEX outbox_aggregate_version ON outbox_events(aggregate_id, aggregate_version);
CREATE TABLE inbox_receipts (
    consumer_name text NOT NULL,
    event_id uuid NOT NULL,
    processed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (consumer_name, event_id)
);

ALTER TABLE submissions ADD COLUMN generation bigint NOT NULL DEFAULT 1 CHECK (generation > 0);
CREATE TABLE judge_jobs (
    id uuid PRIMARY KEY,
    submission_id uuid NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
    generation bigint NOT NULL CHECK (generation > 0),
    package_id uuid REFERENCES problem_packages(id) ON DELETE SET NULL,
    package_hash text NOT NULL,
    state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','processing','completed')),
    attempt integer NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    lease_token uuid,
    deadline_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    UNIQUE (submission_id, generation)
);
CREATE INDEX judge_jobs_recovery ON judge_jobs(deadline_at) WHERE state='processing';

-- +goose Down
DROP TABLE judge_jobs;
ALTER TABLE submissions DROP COLUMN generation;
DROP TABLE inbox_receipts;
DROP INDEX outbox_aggregate_version;
ALTER TABLE outbox_events DROP COLUMN aggregate_version, DROP COLUMN lease_token, DROP COLUMN next_attempt_at;
DROP TABLE event_aggregates;
