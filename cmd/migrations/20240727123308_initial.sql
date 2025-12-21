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
CREATE TYPE user_role AS ENUM ('user', 'admin');
CREATE TABLE IF NOT EXISTS users
(
    id         uuid PRIMARY KEY,
    username   varchar(70)         NOT NULL,
    role       user_role           NOT NULL DEFAULT 'user',
    kratos_id  varchar(255) UNIQUE NOT NULL,
    created_at timestamptz         NOT NULL DEFAULT now(),
    updated_at timestamptz         NOT NULL DEFAULT now(),
    CHECK (length(username) != 0),
    CHECK (
        role = 'user'
            OR role = 'admin'
        )
);
CREATE INDEX IF NOT EXISTS users_username_trgm_idx ON users USING gin (username gin_trgm_ops);
CREATE INDEX IF NOT EXISTS users_kratos_id_idx ON users (kratos_id);
CREATE INDEX IF NOT EXISTS users_role ON users (role);
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
    created_by         uuid               REFERENCES users (id) ON DELETE
        SET NULL,
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
    CHECK (
        memory_limit BETWEEN 4 AND 1024
        ),
    CHECK (
        time_limit BETWEEN 250 AND 5000
        )
);
CREATE INDEX IF NOT EXISTS problem_title_trgm_idx ON problems USING gin (title gin_trgm_ops);
CREATE TRIGGER on_problems_update
    BEFORE
        UPDATE
    ON problems
    FOR EACH ROW
EXECUTE FUNCTION updated_at_update();
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
    created_by               uuid               REFERENCES users (id) ON DELETE
        SET NULL,
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
    contest_id  uuid             REFERENCES contests (id) ON DELETE
        SET NULL,
    problem_id  uuid             REFERENCES problems (id) ON DELETE
        SET NULL,
    created_by  uuid             REFERENCES users (id) ON DELETE
        SET NULL,
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
CREATE TRIGGER on_solutions_update
    BEFORE
        UPDATE
    ON submissions
    FOR EACH ROW
EXECUTE FUNCTION updated_at_update();
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS on_solutions_update ON submissions;
DROP TABLE IF EXISTS submissions;
DROP TABLE IF EXISTS contest_members;
DROP TRIGGER IF EXISTS max_problems_on_contest_check ON contest_problem;
DROP TABLE IF EXISTS contest_problem;
DROP TABLE IF EXISTS problem_members;
DROP TRIGGER IF EXISTS on_problems_update ON problems;
DROP INDEX IF EXISTS problem_title_trgm_idx;
DROP TABLE IF EXISTS problems;
DROP TRIGGER IF EXISTS on_contests_update ON contests;
DROP INDEX IF EXISTS contest_title_trgm_idx;
DROP TABLE IF EXISTS contests;
DROP TRIGGER IF EXISTS on_users_update ON users;
DROP INDEX IF EXISTS users_username_trgm_idx;
DROP INDEX IF EXISTS users_kratos_id_idx;
DROP INDEX IF EXISTS users_role;
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS problem_role;
DROP TYPE IF EXISTS user_role;
DROP FUNCTION IF EXISTS check_max_problems_on_contest();
DROP FUNCTION IF EXISTS updated_at_update();
DROP TYPE IF EXISTS contest_role;
DROP TYPE IF EXISTS contest_visibility;
-- +goose StatementEnd