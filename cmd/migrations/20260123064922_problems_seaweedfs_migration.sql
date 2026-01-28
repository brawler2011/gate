-- +goose Up
-- +goose StatementBegin

-- ============================================================================
-- ПОЛНАЯ МИГРАЦИЯ GATE149 С НОВОЙ АРХИТЕКТУРОЙ PROBLEMS (SeaweedFS)
-- ============================================================================
--
-- Это ПОЛНАЯ замена initial миграции 20240727123308
-- Включает ВСЕ таблицы системы с новой структурой problems
--
-- ВАЖНО: Старые данные НЕ сохраняются! Это fresh start.
--

-- ----------------------------------------------------------------------------
-- 1. EXTENSIONS
-- ----------------------------------------------------------------------------
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ----------------------------------------------------------------------------
-- 2. FUNCTIONS
-- ----------------------------------------------------------------------------

-- Функция для автоматического обновления updated_at
CREATE FUNCTION updated_at_update() RETURNS TRIGGER
    LANGUAGE plpgsql AS
$$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

-- Функция проверки максимального количества задач в контесте (50)
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

-- ----------------------------------------------------------------------------
-- 3. IMAGES TABLE
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS images
(
    id         uuid primary key,
    image      text        not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

CREATE INDEX IF NOT EXISTS images_id_idx ON images (id);

CREATE TRIGGER on_images_update
    BEFORE UPDATE
    ON images
    FOR EACH ROW
EXECUTE FUNCTION updated_at_update();

-- ----------------------------------------------------------------------------
-- 4. USERS TABLE
-- ----------------------------------------------------------------------------
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
    BEFORE UPDATE
    ON users
    FOR EACH ROW
EXECUTE FUNCTION updated_at_update();

-- ----------------------------------------------------------------------------
-- 5. PROBLEMS TABLE (НОВАЯ СТРУКТУРА - SeaweedFS)
-- ----------------------------------------------------------------------------
CREATE TYPE problem_visibility AS ENUM ('private', 'public');
CREATE TYPE problem_type AS ENUM ('pass-fail', 'scoring', 'interactive', 'multi-pass', 'submit-answer');

CREATE TABLE problems
(
    id         uuid PRIMARY KEY             DEFAULT uuid_generate_v7(),

    owner_id   uuid                REFERENCES users (id) ON DELETE SET NULL,
    visibility problem_visibility  NOT NULL DEFAULT 'private',

    -- Мультиязычные названия: {"en": "Sum", "ru": "Сумма"}
    titles     jsonb               NOT NULL,

    -- Короткое имя для URL (уникальное)
    short_name varchar(100) UNIQUE NOT NULL,

    created_at timestamptz         NOT NULL DEFAULT now(),
    updated_at timestamptz         NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS problem_title_trgm_idx ON problems USING gin (title.ru gin_trgm_ops);

CREATE TRIGGER on_problems_update
    BEFORE UPDATE
    ON problems
    FOR EACH ROW
EXECUTE FUNCTION updated_at_update();

-- ----------------------------------------------------------------------------
-- 6. PROBLEM MEMBERS TABLE
-- ----------------------------------------------------------------------------
CREATE TYPE problem_role as ENUM ('owner', 'moderator');

CREATE TABLE problem_members
(
    problem_id uuid REFERENCES problems (id) ON DELETE CASCADE,
    user_id    uuid         NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role       problem_role NOT NULL DEFAULT 'moderator',
    UNIQUE (user_id, problem_id)
);

CREATE INDEX problem_members_problem_id_idx ON problem_members (problem_id);
CREATE INDEX problem_members_user_id_idx ON problem_members (user_id);

-- ----------------------------------------------------------------------------
-- 7. CONTESTS TABLE (БЕЗ ИЗМЕНЕНИЙ от 2024)
-- ----------------------------------------------------------------------------
CREATE TYPE contest_visibility AS ENUM ('private', 'public');
CREATE TYPE contest_role AS ENUM ('owner', 'moderator', 'participant');

CREATE TABLE IF NOT EXISTS contests
(
    id                       uuid PRIMARY KEY            DEFAULT uuid_generate_v7(),
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
    BEFORE UPDATE
    ON contests
    FOR EACH ROW
EXECUTE FUNCTION updated_at_update();

-- ----------------------------------------------------------------------------
-- 8. CONTEST_PROBLEM TABLE (ГИБРИДНАЯ СТРУКТУРА)
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS contest_problem
(
    problem_id uuid REFERENCES problems (id) ON DELETE CASCADE,
    contest_id uuid REFERENCES contests (id) ON DELETE CASCADE,
    ordinal    integer      NOT NULL,
    
    -- Hash версии задачи из SeaweedFS (иммутабельная версия)
    package_hash varchar(64) NOT NULL,
    
    UNIQUE (problem_id, contest_id),
    UNIQUE (contest_id, ordinal),
    CHECK (ordinal >= 0)
);

CREATE TRIGGER max_problems_on_contest_check
    BEFORE INSERT
    ON contest_problem
    FOR EACH ROW
EXECUTE FUNCTION check_max_problems_on_contest();

-- ----------------------------------------------------------------------------
-- 9. CONTEST MEMBERS TABLE
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS contest_members
(
    user_id    uuid         NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    contest_id uuid         NOT NULL REFERENCES contests (id) ON DELETE CASCADE,
    role       contest_role NOT NULL DEFAULT 'participant',
    UNIQUE (user_id, contest_id)
);

-- ----------------------------------------------------------------------------
-- 10. SUBMISSIONS TABLE
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS submissions
(
    id          uuid PRIMARY KEY          DEFAULT uuid_generate_v7(),
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
    BEFORE UPDATE
    ON submissions
    FOR EACH ROW
EXECUTE FUNCTION updated_at_update();

-- ----------------------------------------------------------------------------
-- 11. OUTBOX EVENTS TABLE
-- ----------------------------------------------------------------------------
CREATE TYPE outbox_event_status AS ENUM ('pending', 'processing', 'completed', 'failed');

CREATE TABLE IF NOT EXISTS outbox_events
(
    id            uuid PRIMARY KEY             DEFAULT uuid_generate_v7(),
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

-- ----------------------------------------------------------------------------
-- ПОЛНЫЙ ОТКАТ В ОБРАТНОМ ПОРЯДКЕ
-- ----------------------------------------------------------------------------

-- Drop outbox_events
DROP INDEX IF EXISTS outbox_events_aggregate_id_idx;
DROP INDEX IF EXISTS outbox_events_worker_idx;
DROP TABLE IF EXISTS outbox_events;
DROP TYPE IF EXISTS outbox_event_status;

-- Drop submissions
DROP TRIGGER IF EXISTS on_submissions_update ON submissions;
DROP INDEX IF EXISTS submissions_created_at_idx;
DROP INDEX IF EXISTS submissions_state_idx;
DROP INDEX IF EXISTS submissions_created_by_idx;
DROP INDEX IF EXISTS submissions_problem_id_idx;
DROP INDEX IF EXISTS submissions_contest_id_idx;
DROP TABLE IF EXISTS submissions;

-- Drop contest_members
DROP TABLE IF EXISTS contest_members;

-- Drop contest_problem
DROP TRIGGER IF EXISTS max_problems_on_contest_check ON contest_problem;
DROP INDEX IF EXISTS contest_problem_contest_id_ordinal_idx;
DROP TABLE IF EXISTS contest_problem;

-- Drop contests
DROP TRIGGER IF EXISTS on_contests_update ON contests;
DROP INDEX IF EXISTS contest_title_trgm_idx;
DROP TABLE IF EXISTS contests;
DROP TYPE IF EXISTS contest_role;
DROP TYPE IF EXISTS contest_visibility;

-- Drop problem_members
DROP INDEX IF EXISTS problem_members_user_id_idx;
DROP INDEX IF EXISTS problem_members_problem_id_idx;
DROP TABLE IF EXISTS problem_members;
DROP TYPE IF EXISTS problem_role;

-- Drop problems
DROP TRIGGER IF EXISTS on_problems_update ON problems;
DROP INDEX IF EXISTS problems_created_at_idx;
DROP INDEX IF EXISTS problems_visibility_idx;
DROP INDEX IF EXISTS problems_short_name_idx;
DROP INDEX IF EXISTS problems_owner_id_idx;
DROP TABLE IF EXISTS problems;
DROP TYPE IF EXISTS problem_type;
DROP TYPE IF EXISTS problem_visibility;

-- Drop users
DROP TRIGGER IF EXISTS on_users_update ON users;
DROP INDEX IF EXISTS users_email_idx;
DROP INDEX IF EXISTS users_role;
DROP INDEX IF EXISTS users_kratos_id_idx;
DROP INDEX IF EXISTS users_username_trgm_idx;
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS user_role;

-- Drop images
DROP TRIGGER IF EXISTS on_images_update ON images;
DROP INDEX IF EXISTS images_id_idx;
DROP TABLE IF EXISTS images;

-- Drop functions
DROP FUNCTION IF EXISTS check_max_problems_on_contest();
DROP FUNCTION IF EXISTS updated_at_update();

-- Drop extensions
DROP EXTENSION IF EXISTS pg_trgm;
DROP EXTENSION IF EXISTS "uuid-ossp";

-- +goose StatementEnd
