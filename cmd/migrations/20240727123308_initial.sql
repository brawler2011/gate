-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE FUNCTION updated_at_update() RETURNS TRIGGER
    LANGUAGE plpgsql AS
$$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

CREATE FUNCTION check_max_problems_on_contest() RETURNS TRIGGER
    LANGUAGE plpgsql AS
$$
DECLARE
    max_problems_on_contest_count integer := 50;
BEGIN
    IF (SELECT count(*)
        FROM contest_problem
        WHERE contest_id = NEW.contest_id) >= max_problems_on_contest_count THEN
        RAISE EXCEPTION 'Exceeded max problems for this contest';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TABLE IF NOT EXISTS images
(
    id         uuid primary key,
    image      text        not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);
CREATE INDEX IF NOT EXISTS images_id_idx ON images (id);
CREATE TRIGGER on_images_update
    BEFORE
        UPDATE
    ON images
    FOR EACH ROW
EXECUTE FUNCTION updated_at_update();

CREATE TYPE user_role AS ENUM ('user', 'admin');
CREATE TABLE IF NOT EXISTS users
(
    id         uuid PRIMARY KEY,
    username   varchar(70)  NOT NULL,
    role       user_role    NOT NULL DEFAULT 'user',
    kratos_id  uuid UNIQUE  NOT NULL,
    email      VARCHAR(255) NOT NULL,
    name       VARCHAR(100) NOT NULL,
    surname    VARCHAR(100) NOT NULL,
    bio        VARCHAR(500) NOT NULL,
    img_id     uuid REFERENCES images (id),
    created_at timestamptz  NOT NULL DEFAULT now(),
    updated_at timestamptz  NOT NULL DEFAULT now(),
    CHECK (length(username) != 0)
);
CREATE INDEX IF NOT EXISTS users_username_trgm_idx ON users USING gin (username gin_trgm_ops);
CREATE INDEX IF NOT EXISTS users_kratos_id_idx ON users (kratos_id);
CREATE INDEX IF NOT EXISTS users_role ON users (role);
CREATE UNIQUE INDEX IF NOT EXISTS users_email_idx ON users (email)
    WHERE email IS NOT NULL;
CREATE TRIGGER on_users_update
    BEFORE
        UPDATE
    ON users
    FOR EACH ROW
EXECUTE FUNCTION updated_at_update();

CREATE TYPE problem_visibility AS ENUM ('private', 'public');
CREATE TABLE IF NOT EXISTS problems
(
    id                 uuid PRIMARY KEY            DEFAULT uuid_generate_v4(),
    created_by         uuid               REFERENCES users (id) ON DELETE SET NULL,
    visibility         problem_visibility NOT NULL DEFAULT 'private',
    title              varchar(64)        NOT NULL,
    time_limit         integer            NOT NULL DEFAULT 1000,
    memory_limit       integer            NOT NULL DEFAULT 64,
    legend             varchar(10240)     NOT NULL DEFAULT '',
    input_format       varchar(10240)     NOT NULL DEFAULT '',
    output_format      varchar(10240)     NOT NULL DEFAULT '',
    notes              varchar(10240)     NOT NULL DEFAULT '',
    scoring            varchar(10240)     NOT NULL DEFAULT '',
    legend_html        varchar(10240)     NOT NULL DEFAULT '',
    input_format_html  varchar(10240)     NOT NULL DEFAULT '',
    output_format_html varchar(10240)     NOT NULL DEFAULT '',
    notes_html         varchar(10240)     NOT NULL DEFAULT '',
    scoring_html       varchar(10240)     NOT NULL DEFAULT '',
    created_at         timestamptz        NOT NULL DEFAULT now(),
    updated_at         timestamptz        NOT NULL DEFAULT now(),
    CHECK (length(title) != 0),
    CHECK (memory_limit BETWEEN 4 AND 1024),
    CHECK (time_limit BETWEEN 250 AND 5000)
);
CREATE INDEX IF NOT EXISTS problem_title_trgm_idx ON problems USING gin (title gin_trgm_ops);
CREATE TRIGGER on_problems_update
    BEFORE
        UPDATE
    ON problems
    FOR EACH ROW
EXECUTE FUNCTION updated_at_update();

CREATE TABLE IF NOT EXISTS problem_tests
(
    id         uuid PRIMARY KEY     DEFAULT uuid_generate_v4(),
    problem_id uuid        NOT NULL REFERENCES problems (id) ON DELETE CASCADE,
    ordinal    integer     NOT NULL,
    input      text        NOT NULL DEFAULT '',
    output     text        NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (problem_id, ordinal),
    CHECK (ordinal > 0)
);
CREATE INDEX IF NOT EXISTS problem_tests_problem_id_idx ON problem_tests (problem_id, ordinal);

CREATE TYPE problem_role as ENUM ('owner', 'moderator');
CREATE TABLE IF NOT EXISTS problem_members
(
    problem_id uuid REFERENCES problems (id) ON DELETE CASCADE,
    user_id    uuid         NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role       problem_role NOT NULL DEFAULT 'moderator',
    UNIQUE (user_id, problem_id)
);

CREATE TYPE contest_visibility AS ENUM ('private', 'public');
CREATE TYPE contest_role AS ENUM ('owner', 'moderator', 'participant');
CREATE TABLE IF NOT EXISTS contests
(
    id                       uuid PRIMARY KEY            DEFAULT uuid_generate_v4(),
    title                    varchar(64)        NOT NULL,
    description              varchar(2048)      NOT NULL DEFAULT '',
    visibility               contest_visibility NOT NULL DEFAULT 'private',
    monitor_scope            contest_role       NOT NULL DEFAULT 'participant',
    submissions_list_scope   contest_role       NOT NULL DEFAULT 'moderator',
    submissions_review_scope contest_role       NOT NULL DEFAULT 'moderator',
    created_by               uuid               REFERENCES users (id) ON DELETE SET NULL,
    created_at               timestamptz        NOT NULL DEFAULT now(),
    updated_at               timestamptz        NOT NULL DEFAULT now(),
    CONSTRAINT contest_title_check CHECK (length(title) != 0)
);
CREATE INDEX IF NOT EXISTS contest_title_trgm_idx ON contests USING gin (title gin_trgm_ops);
CREATE TRIGGER on_contests_update
    BEFORE
        UPDATE
    ON contests
    FOR EACH ROW
EXECUTE FUNCTION updated_at_update();

CREATE TABLE IF NOT EXISTS contest_problem
(
    problem_id uuid REFERENCES problems (id) ON DELETE CASCADE,
    contest_id uuid REFERENCES contests (id) ON DELETE CASCADE,
    position   integer NOT NULL,
    UNIQUE (problem_id, contest_id),
    UNIQUE (contest_id, position),
    CHECK (position >= 0)
);
CREATE TRIGGER max_problems_on_contest_check
    BEFORE
        INSERT
    ON contest_problem
    FOR EACH ROW
EXECUTE FUNCTION check_max_problems_on_contest();

CREATE TABLE IF NOT EXISTS contest_members
(
    user_id    uuid         NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    contest_id uuid         NOT NULL REFERENCES contests (id) ON DELETE CASCADE,
    role       contest_role NOT NULL DEFAULT 'participant',
    UNIQUE (user_id, contest_id)
);

CREATE TABLE IF NOT EXISTS submissions
(
    id          uuid PRIMARY KEY          DEFAULT uuid_generate_v4(),
    contest_id  uuid             REFERENCES contests (id) ON DELETE SET NULL,
    problem_id  uuid             REFERENCES problems (id) ON DELETE SET NULL,
    created_by  uuid             REFERENCES users (id) ON DELETE SET NULL,
    submission  varchar(1048576) NOT NULL,
    language    integer          NOT NULL,
    state       integer          NOT NULL DEFAULT 1,
    score       integer          NOT NULL DEFAULT 0,
    penalty     integer          NOT NULL,
    time_stat   integer          NOT NULL DEFAULT 0,
    memory_stat integer          NOT NULL DEFAULT 0,
    updated_at  timestamptz      NOT NULL DEFAULT now(),
    created_at  timestamptz      NOT NULL DEFAULT now()
);
CREATE TRIGGER on_submissions_update
    BEFORE
        UPDATE
    ON submissions
    FOR EACH ROW
EXECUTE FUNCTION updated_at_update();

CREATE TYPE outbox_event_status AS ENUM ('pending', 'processing', 'completed', 'failed');
CREATE TABLE IF NOT EXISTS outbox_events
(
    id            uuid PRIMARY KEY             DEFAULT uuid_generate_v4(),
    aggregate_id  uuid                NOT NULL,
    event_type    varchar(64)         NOT NULL,
    payload       jsonb               NOT NULL DEFAULT '{}',
    status        outbox_event_status NOT NULL DEFAULT 'pending',
    retry_count   integer             NOT NULL DEFAULT 0,
    error_message text,

    -- Event lifecycle fields
    created_at    timestamptz         NOT NULL DEFAULT now(),
    processed_at  timestamptz,
    locked_at     timestamptz,
    deadline_at   timestamptz,

    CHECK (retry_count >= 0)
);
CREATE INDEX IF NOT EXISTS outbox_events_worker_idx
    ON outbox_events (created_at)
    WHERE status = 'pending' OR status = 'processing';
CREATE INDEX IF NOT EXISTS outbox_events_aggregate_id_idx
    ON outbox_events (aggregate_id);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS outbox_events_aggregate_idx;
DROP INDEX IF EXISTS outbox_events_status_created_at_idx;
DROP TABLE IF EXISTS outbox_events;
DROP TYPE IF EXISTS outbox_event_status;
DROP TRIGGER IF EXISTS on_submissions_update ON submissions;
DROP TABLE IF EXISTS submissions;
DROP TABLE IF EXISTS contest_members;
DROP TRIGGER IF EXISTS max_problems_on_contest_check ON contest_problem;
DROP TABLE IF EXISTS contest_problem;
DROP TRIGGER IF EXISTS on_contests_update ON contests;
DROP INDEX IF EXISTS contest_title_trgm_idx;
DROP TABLE IF EXISTS contests;
DROP TYPE IF EXISTS contest_role;
DROP TYPE IF EXISTS contest_visibility;
DROP TABLE IF EXISTS problem_members;
DROP TYPE IF EXISTS problem_role;
DROP INDEX IF EXISTS problem_tests_problem_id_idx;
DROP TABLE IF EXISTS problem_tests;
DROP TRIGGER IF EXISTS on_problems_update ON problems;
DROP INDEX IF EXISTS problem_title_trgm_idx;
DROP TABLE IF EXISTS problems;
DROP TYPE IF EXISTS problem_visibility;
DROP TRIGGER IF EXISTS on_users_update ON users;
DROP INDEX IF EXISTS users_email_idx;
DROP INDEX IF EXISTS users_role;
DROP INDEX IF EXISTS users_kratos_id_idx;
DROP INDEX IF EXISTS users_username_trgm_idx;
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS user_role;
DROP TRIGGER IF EXISTS on_images_update ON images;
DROP INDEX IF EXISTS images_id_idx;
DROP TABLE IF EXISTS images;
DROP FUNCTION IF EXISTS check_max_problems_on_contest();
DROP FUNCTION IF EXISTS updated_at_update();
-- +goose StatementEnd