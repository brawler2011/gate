-- +goose Up
-- +goose StatementBegin

-- ============================================================================
-- EXTENSIONS
-- ============================================================================
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ============================================================================
-- FUNCTIONS
-- ============================================================================

-- Generate UUID v7 (time-ordered UUID with millisecond precision)
-- Format: 48-bit timestamp | 12-bit random | 2-bit version | 62-bit random
CREATE FUNCTION uuid_generate_v7() RETURNS UUID
    LANGUAGE plpgsql
AS $$
DECLARE
    unix_ts_ms BIGINT;
    uuid_bytes BYTEA;
BEGIN
    -- Get Unix timestamp in milliseconds
    unix_ts_ms := (EXTRACT(EPOCH FROM clock_timestamp()) * 1000)::BIGINT;
    
    -- Generate UUID v7
    uuid_bytes := 
        -- 48-bit timestamp
        substring(int8send(unix_ts_ms) from 3 for 6) ||
        -- 12-bit random + 4-bit version (0x7)
        substring(gen_random_bytes(2) from 1 for 2) ||
        -- 2-bit variant (0b10) + 62-bit random
        substring(gen_random_bytes(8) from 1 for 8);
    
    -- Set version (7) in bits 48-51
    uuid_bytes := set_byte(uuid_bytes, 6, (get_byte(uuid_bytes, 6) & 15) | 112);
    
    -- Set variant (RFC 4122) in bits 64-65
    uuid_bytes := set_byte(uuid_bytes, 8, (get_byte(uuid_bytes, 8) & 63) | 128);
    
    RETURN encode(uuid_bytes, 'hex')::UUID;
END;
$$;

-- Auto-update updated_at timestamp on row update
CREATE FUNCTION updated_at_update() RETURNS TRIGGER
    LANGUAGE plpgsql AS
$$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

-- Enforce maximum 50 problems per contest
CREATE FUNCTION check_max_problems_on_contest() RETURNS TRIGGER
    LANGUAGE plpgsql AS
$$
DECLARE
    max_problems_on_contest_count integer := 50;
BEGIN
    IF (SELECT count(*)
        FROM contest_problems
        WHERE contest_id = NEW.contest_id) >= max_problems_on_contest_count THEN
        RAISE EXCEPTION 'Exceeded max problems for this contest (limit: %)', max_problems_on_contest_count;
    END IF;
    RETURN NEW;
END;
$$;

-- Check if user has access to problem (with organization inheritance)
-- Returns true if user has access via: direct membership, team membership, or organization ownership/admin role
CREATE FUNCTION user_has_problem_access(p_user_id UUID, p_problem_id UUID) RETURNS BOOLEAN
    LANGUAGE plpgsql STABLE AS
$$
DECLARE
    v_organization_id UUID;
    v_has_access      BOOLEAN;
BEGIN
    -- Get problem's organization
    SELECT organization_id INTO v_organization_id FROM problems WHERE id = p_problem_id;
    IF NOT FOUND THEN
        RETURN FALSE;
    END IF;

    -- Check if user is owner/admin of organization
    SELECT EXISTS(SELECT 1
                  FROM organization_members
                  WHERE organization_id = v_organization_id
                    AND user_id = p_user_id
                    AND role IN ('owner', 'admin')) INTO v_has_access;
    IF v_has_access THEN
        RETURN TRUE;
    END IF;

    -- Check direct problem membership
    SELECT EXISTS(SELECT 1 FROM problem_members WHERE problem_id = p_problem_id AND user_id = p_user_id)
    INTO v_has_access;
    IF v_has_access THEN
        RETURN TRUE;
    END IF;

    -- Check team-based access
    SELECT EXISTS(SELECT 1
                  FROM problem_teams pt
                           INNER JOIN team_members tm ON pt.team_id = tm.team_id
                  WHERE pt.problem_id = p_problem_id
                    AND tm.user_id = p_user_id) INTO v_has_access;

    RETURN v_has_access;
END;
$$;

-- Check if user has access to contest (with organization inheritance)
CREATE FUNCTION user_has_contest_access(p_user_id UUID, p_contest_id UUID) RETURNS BOOLEAN
    LANGUAGE plpgsql STABLE AS
$$
DECLARE
    v_organization_id UUID;
    v_has_access      BOOLEAN;
BEGIN
    -- Get contest's organization
    SELECT organization_id INTO v_organization_id FROM contests WHERE id = p_contest_id;
    IF NOT FOUND THEN
        RETURN FALSE;
    END IF;

    -- Check if user is owner/admin of organization
    SELECT EXISTS(SELECT 1
                  FROM organization_members
                  WHERE organization_id = v_organization_id
                    AND user_id = p_user_id
                    AND role IN ('owner', 'admin')) INTO v_has_access;
    IF v_has_access THEN
        RETURN TRUE;
    END IF;

    -- Check direct contest membership
    SELECT EXISTS(SELECT 1 FROM contest_members WHERE contest_id = p_contest_id AND user_id = p_user_id)
    INTO v_has_access;
    IF v_has_access THEN
        RETURN TRUE;
    END IF;

    -- Check team-based access
    SELECT EXISTS(SELECT 1
                  FROM contest_teams ct
                           INNER JOIN team_members tm ON ct.team_id = tm.team_id
                  WHERE ct.contest_id = p_contest_id
                    AND tm.user_id = p_user_id) INTO v_has_access;

    RETURN v_has_access;
END;
$$;

-- Check if user is moderator/owner of contest (for realtime scoreboard access)
CREATE FUNCTION user_is_contest_moderator(p_user_id UUID, p_contest_id UUID) RETURNS BOOLEAN
    LANGUAGE plpgsql STABLE AS
$$
DECLARE
    v_organization_id UUID;
    v_is_moderator    BOOLEAN;
BEGIN
    -- Get contest's organization
    SELECT organization_id INTO v_organization_id FROM contests WHERE id = p_contest_id;
    IF NOT FOUND THEN
        RETURN FALSE;
    END IF;

    -- Check if user is owner/admin of organization
    SELECT EXISTS(SELECT 1
                  FROM organization_members
                  WHERE organization_id = v_organization_id
                    AND user_id = p_user_id
                    AND role IN ('owner', 'admin')) INTO v_is_moderator;
    IF v_is_moderator THEN
        RETURN TRUE;
    END IF;

    -- Check if user is contest owner or moderator
    SELECT EXISTS(SELECT 1
                  FROM contest_members
                  WHERE contest_id = p_contest_id
                    AND user_id = p_user_id
                    AND role IN ('owner', 'moderator')) INTO v_is_moderator;

    RETURN v_is_moderator;
END;
$$;

-- Get contest scoreboard with freeze support
-- Returns aggregated results per user with problem-level statistics
-- p_realtime: if true, shows all results even during freeze (for moderators)
CREATE FUNCTION get_contest_scoreboard(
    p_contest_id UUID,
    p_realtime BOOLEAN DEFAULT FALSE
) RETURNS TABLE (
    user_id UUID,
    username VARCHAR(70),
    user_name VARCHAR(100),
    user_surname VARCHAR(100),
    problems_solved INTEGER,
    total_penalty INTEGER,
    last_accepted_at TIMESTAMPTZ,
    problem_results JSONB
)
LANGUAGE plpgsql STABLE AS
$$
DECLARE
    v_freeze_time TIMESTAMPTZ;
    v_contest_start TIMESTAMPTZ;
    v_contest_end TIMESTAMPTZ;
BEGIN
    -- Get contest settings and times
    SELECT 
        start_time,
        end_time,
        CASE 
            WHEN NOT p_realtime 
                AND (settings->>'freeze_monitor')::boolean IS TRUE
                AND end_time IS NOT NULL
            THEN end_time - COALESCE(
                (settings->>'freeze_duration')::interval,
                INTERVAL '1 hour'
            )
            ELSE NULL
        END
    INTO v_contest_start, v_contest_end, v_freeze_time
    FROM contests 
    WHERE id = p_contest_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Contest not found: %', p_contest_id;
    END IF;

    RETURN QUERY
    WITH ranked_submissions AS (
        SELECT 
            s.owner_id,
            s.problem_id,
            s.state,
            s.score,
            s.penalty,
            s.created_at,
            -- Apply freeze: hide results after freeze_time for non-realtime
            CASE 
                WHEN v_freeze_time IS NOT NULL AND s.created_at >= v_freeze_time 
                THEN NULL  -- Frozen state
                ELSE s.state
            END as visible_state,
            CASE 
                WHEN v_freeze_time IS NOT NULL AND s.created_at >= v_freeze_time 
                THEN NULL
                ELSE s.score
            END as visible_score,
            ROW_NUMBER() OVER (
                PARTITION BY s.owner_id, s.problem_id 
                ORDER BY s.created_at ASC
            ) as attempt_num
        FROM submissions s
        WHERE s.contest_id = p_contest_id
          AND s.owner_id IS NOT NULL
    ),
    problem_stats AS (
        SELECT 
            rs.owner_id,
            rs.problem_id,
            COUNT(*) as attempts,
            -- Count frozen attempts (submissions after freeze)
            COUNT(*) FILTER (WHERE rs.visible_state IS NULL) as frozen_attempts,
            -- First accepted submission time
            MIN(CASE WHEN rs.visible_state = 3 THEN rs.created_at END) as first_ac_time,
            -- Calculate penalty: sum of penalties for wrong attempts before AC
            COALESCE(SUM(
                CASE 
                    WHEN rs.visible_state = 3 THEN 0
                    WHEN MIN(CASE WHEN rs.visible_state = 3 THEN rs.created_at END) 
                         OVER (PARTITION BY rs.owner_id, rs.problem_id) IS NOT NULL
                    THEN rs.penalty
                    ELSE 0
                END
            ), 0) as problem_penalty,
            -- Problem is solved if any visible submission is accepted
            BOOL_OR(rs.visible_state = 3) as solved,
            -- Best score among visible submissions
            MAX(rs.visible_score) as best_score
        FROM ranked_submissions rs
        GROUP BY rs.owner_id, rs.problem_id
    ),
    user_stats AS (
        SELECT 
            ps.owner_id,
            COUNT(*) FILTER (WHERE ps.solved) as problems_solved,
            COALESCE(SUM(
                CASE WHEN ps.solved AND v_contest_start IS NOT NULL
                THEN EXTRACT(EPOCH FROM (ps.first_ac_time - v_contest_start))::INTEGER / 60 
                     + ps.problem_penalty
                ELSE 0 END
            ), 0)::INTEGER as total_penalty,
            MAX(ps.first_ac_time) as last_accepted_at,
            jsonb_object_agg(
                ps.problem_id,
                jsonb_build_object(
                    'attempts', ps.attempts,
                    'frozen_attempts', ps.frozen_attempts,
                    'solved', ps.solved,
                    'time_minutes', CASE 
                        WHEN ps.first_ac_time IS NOT NULL AND v_contest_start IS NOT NULL
                        THEN EXTRACT(EPOCH FROM (ps.first_ac_time - v_contest_start))::INTEGER / 60
                        ELSE NULL 
                    END,
                    'penalty', ps.problem_penalty,
                    'score', ps.best_score
                )
            ) as problem_results
        FROM problem_stats ps
        GROUP BY ps.owner_id
    )
    SELECT 
        u.id,
        u.username,
        u.name,
        u.surname,
        COALESCE(us.problems_solved, 0)::INTEGER,
        COALESCE(us.total_penalty, 0)::INTEGER,
        us.last_accepted_at,
        COALESCE(us.problem_results, '{}'::jsonb)
    FROM contest_members cm
    INNER JOIN users u ON u.id = cm.user_id
    LEFT JOIN user_stats us ON us.owner_id = u.id
    WHERE cm.contest_id = p_contest_id
      AND cm.role = 'participant'
    ORDER BY 
        COALESCE(us.problems_solved, 0) DESC,
        COALESCE(us.total_penalty, 0) ASC,
        us.last_accepted_at ASC NULLS LAST,
        u.username ASC;
END;
$$;

-- ============================================================================
-- ENUMS
-- ============================================================================

CREATE TYPE user_role AS ENUM ('user', 'admin');
CREATE TYPE problem_visibility AS ENUM ('private', 'public', 'unlisted');
CREATE TYPE contest_visibility AS ENUM ('private', 'public', 'unlisted');
CREATE TYPE contest_role AS ENUM ('owner', 'moderator', 'participant');
CREATE TYPE problem_role AS ENUM ('owner', 'moderator', 'viewer');
CREATE TYPE problem_permission AS ENUM ('admin', 'write', 'read');
CREATE TYPE package_status AS ENUM ('pending', 'building', 'ready', 'failed');
CREATE TYPE outbox_event_status AS ENUM ('pending', 'processing', 'completed', 'failed');
CREATE TYPE team_role AS ENUM ('maintainer', 'member');
CREATE TYPE organization_role AS ENUM ('owner', 'admin', 'member');

-- ============================================================================
-- ORGANIZATIONS & USERS
-- ============================================================================

-- Organizations: schools, universities, companies
CREATE TABLE organizations
(
    id          UUID PRIMARY KEY             DEFAULT uuid_generate_v7(),
    login       TEXT        NOT NULL UNIQUE,
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    avatar_url  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (length(login) > 0),
    CHECK (length(name) > 0)
);

CREATE INDEX organizations_login_idx ON organizations (login);
CREATE INDEX organizations_created_at_idx ON organizations (created_at DESC);

CREATE TRIGGER on_organizations_update
    BEFORE UPDATE
    ON organizations
    FOR EACH ROW
EXECUTE FUNCTION updated_at_update();

-- Users with Kratos authentication
CREATE TABLE users
(
    id                   UUID PRIMARY KEY,
    username             VARCHAR(70)  NOT NULL UNIQUE,
    role                 user_role    NOT NULL DEFAULT 'user',
    kratos_id            UUID UNIQUE  NOT NULL,
    email                VARCHAR(255) NOT NULL,
    name                 VARCHAR(100) NOT NULL,
    surname              VARCHAR(100) NOT NULL,
    bio                  VARCHAR(500) NOT NULL DEFAULT '',
    avatar_url           TEXT,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CHECK (length(username) > 0)
);

CREATE INDEX users_username_trgm_idx ON users USING GIN (username gin_trgm_ops);
CREATE INDEX users_kratos_id_idx ON users (kratos_id);
CREATE INDEX users_role_idx ON users (role);
CREATE UNIQUE INDEX users_email_idx ON users (email);

CREATE TRIGGER on_users_update
    BEFORE UPDATE
    ON users
    FOR EACH ROW
EXECUTE FUNCTION updated_at_update();

-- ============================================================================
-- TEAMS (GitHub-style nested hierarchy)
-- ============================================================================

CREATE TABLE teams
(
    id              UUID PRIMARY KEY             DEFAULT uuid_generate_v7(),
    organization_id UUID        NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name            TEXT        NOT NULL,
    slug            TEXT        NOT NULL,
    description     TEXT        NOT NULL DEFAULT '',
    privacy         TEXT        NOT NULL DEFAULT 'closed',
    parent_team_id  UUID REFERENCES teams (id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, slug),
    CHECK (length(name) > 0),
    CHECK (length(slug) > 0),
    CHECK (privacy IN ('secret', 'closed'))
);

CREATE INDEX teams_organization_id_idx ON teams (organization_id);
CREATE INDEX teams_parent_team_id_idx ON teams (parent_team_id);
CREATE INDEX teams_slug_idx ON teams (organization_id, slug);

CREATE TRIGGER on_teams_update
    BEFORE UPDATE
    ON teams
    FOR EACH ROW
EXECUTE FUNCTION updated_at_update();

-- ============================================================================
-- ORGANIZATION & TEAM MEMBERSHIP
-- ============================================================================

-- Organization members
CREATE TABLE organization_members
(
    organization_id UUID              NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    user_id         UUID              NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role            organization_role NOT NULL DEFAULT 'member',
    created_at      TIMESTAMPTZ       NOT NULL DEFAULT NOW(),
    PRIMARY KEY (organization_id, user_id)
);

CREATE INDEX organization_members_user_id_idx ON organization_members (user_id);

-- Team members
CREATE TABLE team_members
(
    team_id    UUID        NOT NULL REFERENCES teams (id) ON DELETE CASCADE,
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role       team_role   NOT NULL DEFAULT 'member',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (team_id, user_id)
);

CREATE INDEX team_members_user_id_idx ON team_members (user_id);

-- ============================================================================
-- PROBLEMS (Git Repositories)
-- ============================================================================

-- Problems: Organization-owned repositories with git versioning
-- Working copy stored in local filesystem: /var/gate/problems/{org_id}/{problem_id}/
CREATE TABLE problems
(
    id              UUID PRIMARY KEY             DEFAULT uuid_generate_v7(),
    organization_id UUID                NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    owner_id        UUID REFERENCES users (id) ON DELETE SET NULL,
    visibility      problem_visibility  NOT NULL DEFAULT 'private',
    titles          JSONB               NOT NULL,
    short_name      VARCHAR(100)        NOT NULL,
    git_commit_hash VARCHAR(40),
    created_at      TIMESTAMPTZ         NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ         NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, short_name),
    CHECK (length(short_name) > 0),
    CHECK (jsonb_typeof(titles) = 'object')
);

CREATE INDEX problems_organization_id_idx ON problems (organization_id);
CREATE INDEX problems_owner_id_idx ON problems (owner_id);
CREATE INDEX problems_short_name_idx ON problems (short_name);
CREATE INDEX problems_visibility_idx ON problems (visibility);
CREATE INDEX problems_created_at_idx ON problems (created_at DESC);
CREATE INDEX problems_titles_trgm_idx ON problems USING GIN ((titles->>'en') gin_trgm_ops);

CREATE TRIGGER on_problems_update
    BEFORE UPDATE
    ON problems
    FOR EACH ROW
EXECUTE FUNCTION updated_at_update();

-- ============================================================================
-- PROBLEM PACKAGES (Polygon-style compiled versions)
-- ============================================================================

-- Problem packages: Compiled, immutable versions ready for contests
-- Stored in S3: s3://bucket/packages/{org_id}/{problem_id}/{package_hash}/
-- Contains: compiled binaries, generated tests, validated output
CREATE TABLE problem_packages
(
    id              UUID PRIMARY KEY             DEFAULT uuid_generate_v7(),
    problem_id      UUID             NOT NULL REFERENCES problems (id) ON DELETE CASCADE,
    organization_id UUID             NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    git_commit_hash VARCHAR(40)      NOT NULL,
    package_hash    VARCHAR(64)      NOT NULL UNIQUE,
    url             VARCHAR(512),
    status          package_status   NOT NULL DEFAULT 'pending',
    build_log       TEXT,
    created_at      TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    compiled_at     TIMESTAMPTZ,
    CHECK (length(git_commit_hash) = 40),
    CHECK (length(package_hash) = 64)
);

CREATE INDEX problem_packages_problem_id_idx ON problem_packages (problem_id);
CREATE INDEX problem_packages_organization_id_idx ON problem_packages (organization_id);
CREATE INDEX problem_packages_package_hash_idx ON problem_packages (package_hash);
CREATE INDEX problem_packages_status_idx ON problem_packages (status);
CREATE INDEX problem_packages_created_at_idx ON problem_packages (created_at DESC);

-- ============================================================================
-- PROBLEM PERMISSIONS (Dual: Direct + Team-based)
-- ============================================================================

-- Direct user access to problems
CREATE TABLE problem_members
(
    problem_id UUID         NOT NULL REFERENCES problems (id) ON DELETE CASCADE,
    user_id    UUID         NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role       problem_role NOT NULL DEFAULT 'viewer',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (problem_id, user_id)
);

CREATE INDEX problem_members_user_id_idx ON problem_members (user_id);
CREATE INDEX problem_members_problem_id_idx ON problem_members (problem_id);

-- Team-based access to problems
CREATE TABLE problem_teams
(
    problem_id UUID               NOT NULL REFERENCES problems (id) ON DELETE CASCADE,
    team_id    UUID               NOT NULL REFERENCES teams (id) ON DELETE CASCADE,
    permission problem_permission NOT NULL DEFAULT 'read',
    created_at TIMESTAMPTZ        NOT NULL DEFAULT NOW(),
    PRIMARY KEY (problem_id, team_id)
);

CREATE INDEX problem_teams_team_id_idx ON problem_teams (team_id);
CREATE INDEX problem_teams_problem_id_idx ON problem_teams (problem_id);

-- ============================================================================
-- CONTESTS (Non-Versioned Repositories)
-- ============================================================================

-- Contests: Organization-owned, time-bound competitions
CREATE TABLE contests
(
    id              UUID PRIMARY KEY             DEFAULT uuid_generate_v7(),
    organization_id UUID                NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    owner_id        UUID REFERENCES users (id) ON DELETE SET NULL,
    visibility      contest_visibility  NOT NULL DEFAULT 'private',
    titles          JSONB               NOT NULL,
    short_name      VARCHAR(100)        NOT NULL,
    description     TEXT                NOT NULL DEFAULT '',

    -- Settings: contest behavior configuration
    -- Example: '{
    --   "freeze_monitor": true,
    --   "freeze_duration": "1 hour",
    --   "show_verdicts": true,
    --   "show_test_details": false,
    --   "penalty_per_wrong": 20,
    --   "allow_clarifications": true
    -- }'
    settings        JSONB               NOT NULL,

    -- Access policy: role-based permissions
    -- Example: '{"participant": ["tasks.view", "submissions.create"], "moderator": ["*"]}'
    access_policy   JSONB               NOT NULL,

    start_time      TIMESTAMPTZ,
    end_time        TIMESTAMPTZ,
    created_at      TIMESTAMPTZ         NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ         NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, short_name),
    CHECK (length(short_name) > 0),
    CHECK (jsonb_typeof(titles) = 'object'),
    CHECK (end_time IS NULL OR start_time IS NULL OR end_time > start_time)
);

CREATE INDEX contests_organization_id_idx ON contests (organization_id);
CREATE INDEX contests_owner_id_idx ON contests (owner_id);
CREATE INDEX contests_short_name_idx ON contests (short_name);
CREATE INDEX contests_visibility_idx ON contests (visibility);
CREATE INDEX contests_created_at_idx ON contests (created_at DESC);
CREATE INDEX contests_start_time_idx ON contests (start_time);
CREATE INDEX contests_titles_trgm_idx ON contests USING GIN ((titles->>'en') gin_trgm_ops);

CREATE TRIGGER on_contests_update
    BEFORE UPDATE
    ON contests
    FOR EACH ROW
EXECUTE FUNCTION updated_at_update();

-- ============================================================================
-- CONTEST PERMISSIONS (Dual: Direct + Team-based)
-- ============================================================================

-- Direct user access to contests
CREATE TABLE contest_members
(
    contest_id UUID         NOT NULL REFERENCES contests (id) ON DELETE CASCADE,
    user_id    UUID         NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role       contest_role NOT NULL DEFAULT 'participant',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (contest_id, user_id)
);

CREATE INDEX contest_members_user_id_idx ON contest_members (user_id);
CREATE INDEX contest_members_contest_id_idx ON contest_members (contest_id);

-- Team-based access to contests
CREATE TABLE contest_teams
(
    contest_id UUID         NOT NULL REFERENCES contests (id) ON DELETE CASCADE,
    team_id    UUID         NOT NULL REFERENCES teams (id) ON DELETE CASCADE,
    role       contest_role NOT NULL DEFAULT 'participant',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (contest_id, team_id)
);

CREATE INDEX contest_teams_team_id_idx ON contest_teams (team_id);
CREATE INDEX contest_teams_contest_id_idx ON contest_teams (contest_id);

-- ============================================================================
-- CONTEST-PROBLEM LINKING (References Compiled Packages)
-- ============================================================================

-- Links contests to compiled problem packages
-- Ensures only validated, compiled packages can be added to contests
CREATE TABLE contest_problems
(
    contest_id UUID        NOT NULL REFERENCES contests (id) ON DELETE CASCADE,
    problem_id UUID        NOT NULL REFERENCES problems (id) ON DELETE CASCADE,
    package_id UUID        NOT NULL REFERENCES problem_packages (id) ON DELETE RESTRICT,
    ordinal    INTEGER     NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (contest_id, problem_id),
    UNIQUE (contest_id, ordinal),
    CHECK (ordinal >= 0)
);

CREATE INDEX contest_problems_contest_id_idx ON contest_problems (contest_id);
CREATE INDEX contest_problems_problem_id_idx ON contest_problems (problem_id);
CREATE INDEX contest_problems_package_id_idx ON contest_problems (package_id);
CREATE INDEX contest_problems_ordinal_idx ON contest_problems (contest_id, ordinal);

CREATE TRIGGER max_problems_on_contest_check
    BEFORE INSERT
    ON contest_problems
    FOR EACH ROW
EXECUTE FUNCTION check_max_problems_on_contest();

-- ============================================================================
-- SUBMISSIONS
-- ============================================================================

-- User submissions to problems (in contests or standalone practice)
CREATE TABLE submissions
(
    id          UUID PRIMARY KEY          DEFAULT uuid_generate_v7(),
    contest_id  UUID REFERENCES contests (id) ON DELETE SET NULL,
    problem_id  UUID REFERENCES problems (id) ON DELETE SET NULL,
    owner_id  UUID REFERENCES users (id) ON DELETE SET NULL,
    source      VARCHAR(65536)   NOT NULL,
    language    INTEGER          NOT NULL,
    state       INTEGER          NOT NULL DEFAULT 1,
    score       INTEGER          NOT NULL DEFAULT 0,
    penalty     INTEGER          NOT NULL DEFAULT 0,
    time_stat   INTEGER          NOT NULL DEFAULT 0,
    memory_stat INTEGER          NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ      NOT NULL DEFAULT NOW()
);

CREATE INDEX submissions_contest_id_idx ON submissions (contest_id);
CREATE INDEX submissions_problem_id_idx ON submissions (problem_id);
CREATE INDEX submissions_owner_id_idx ON submissions (owner_id);
CREATE INDEX submissions_state_idx ON submissions (state);
CREATE INDEX submissions_created_at_idx ON submissions (created_at DESC);
CREATE INDEX submissions_contest_problem_idx ON submissions (contest_id, problem_id);

-- Composite indexes for scoreboard queries (monitor optimization)
CREATE INDEX submissions_scoreboard_idx ON submissions (contest_id, owner_id, problem_id, created_at)
    WHERE contest_id IS NOT NULL AND owner_id IS NOT NULL;
CREATE INDEX submissions_scoreboard_state_idx ON submissions (contest_id, owner_id, state, created_at)
    WHERE contest_id IS NOT NULL AND owner_id IS NOT NULL;

CREATE TRIGGER on_submissions_update
    BEFORE UPDATE
    ON submissions
    FOR EACH ROW
EXECUTE FUNCTION updated_at_update();

-- ============================================================================
-- OUTBOX EVENTS (Event Sourcing / Async Processing)
-- ============================================================================

-- Transactional outbox pattern for reliable event processing
CREATE TABLE outbox_events
(
    id            UUID PRIMARY KEY             DEFAULT uuid_generate_v7(),
    aggregate_id  UUID                NOT NULL,
    event_type    VARCHAR(64)         NOT NULL,
    payload       JSONB               NOT NULL DEFAULT '{}',
    status        outbox_event_status NOT NULL DEFAULT 'pending',
    retry_count   INTEGER             NOT NULL DEFAULT 0,
    error_message TEXT,
    created_at    TIMESTAMPTZ         NOT NULL DEFAULT NOW(),
    processed_at  TIMESTAMPTZ,
    locked_at     TIMESTAMPTZ,
    deadline_at   TIMESTAMPTZ,
    CHECK (retry_count >= 0)
);

CREATE INDEX outbox_events_worker_idx
    ON outbox_events (created_at)
    WHERE status = 'pending' OR status = 'processing';
CREATE INDEX outbox_events_aggregate_id_idx ON outbox_events (aggregate_id);
CREATE INDEX outbox_events_status_idx ON outbox_events (status);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Drop tables in reverse dependency order
DROP INDEX IF EXISTS outbox_events_status_idx;
DROP INDEX IF EXISTS outbox_events_aggregate_id_idx;
DROP INDEX IF EXISTS outbox_events_worker_idx;
DROP TABLE IF EXISTS outbox_events;

DROP TRIGGER IF EXISTS on_submissions_update ON submissions;
DROP INDEX IF EXISTS submissions_scoreboard_state_idx;
DROP INDEX IF EXISTS submissions_scoreboard_idx;
DROP INDEX IF EXISTS submissions_contest_problem_idx;
DROP INDEX IF EXISTS submissions_created_at_idx;
DROP INDEX IF EXISTS submissions_state_idx;
DROP INDEX IF EXISTS submissions_owner_id_idx;
DROP INDEX IF EXISTS submissions_problem_id_idx;
DROP INDEX IF EXISTS submissions_contest_id_idx;
DROP TABLE IF EXISTS submissions;

DROP TRIGGER IF EXISTS max_problems_on_contest_check ON contest_problems;
DROP INDEX IF EXISTS contest_problems_ordinal_idx;
DROP INDEX IF EXISTS contest_problems_package_id_idx;
DROP INDEX IF EXISTS contest_problems_problem_id_idx;
DROP INDEX IF EXISTS contest_problems_contest_id_idx;
DROP TABLE IF EXISTS contest_problems;

DROP INDEX IF EXISTS contest_teams_contest_id_idx;
DROP INDEX IF EXISTS contest_teams_team_id_idx;
DROP TABLE IF EXISTS contest_teams;

DROP INDEX IF EXISTS contest_members_contest_id_idx;
DROP INDEX IF EXISTS contest_members_user_id_idx;
DROP TABLE IF EXISTS contest_members;

DROP TRIGGER IF EXISTS on_contests_update ON contests;
DROP INDEX IF EXISTS contests_titles_trgm_idx;
DROP INDEX IF EXISTS contests_start_time_idx;
DROP INDEX IF EXISTS contests_created_at_idx;
DROP INDEX IF EXISTS contests_visibility_idx;
DROP INDEX IF EXISTS contests_short_name_idx;
DROP INDEX IF EXISTS contests_owner_id_idx;
DROP INDEX IF EXISTS contests_organization_id_idx;
DROP TABLE IF EXISTS contests;

DROP INDEX IF EXISTS problem_teams_problem_id_idx;
DROP INDEX IF EXISTS problem_teams_team_id_idx;
DROP TABLE IF EXISTS problem_teams;

DROP INDEX IF EXISTS problem_members_problem_id_idx;
DROP INDEX IF EXISTS problem_members_user_id_idx;
DROP TABLE IF EXISTS problem_members;

DROP INDEX IF EXISTS problem_packages_created_at_idx;
DROP INDEX IF EXISTS problem_packages_status_idx;
DROP INDEX IF EXISTS problem_packages_package_hash_idx;
DROP INDEX IF EXISTS problem_packages_organization_id_idx;
DROP INDEX IF EXISTS problem_packages_problem_id_idx;
DROP TABLE IF EXISTS problem_packages;

DROP TRIGGER IF EXISTS on_problems_update ON problems;
DROP INDEX IF EXISTS problems_titles_trgm_idx;
DROP INDEX IF EXISTS problems_created_at_idx;
DROP INDEX IF EXISTS problems_visibility_idx;
DROP INDEX IF EXISTS problems_short_name_idx;
DROP INDEX IF EXISTS problems_owner_id_idx;
DROP INDEX IF EXISTS problems_organization_id_idx;
DROP TABLE IF EXISTS problems;

DROP INDEX IF EXISTS team_members_user_id_idx;
DROP TABLE IF EXISTS team_members;

DROP INDEX IF EXISTS organization_members_user_id_idx;
DROP TABLE IF EXISTS organization_members;

DROP TRIGGER IF EXISTS on_teams_update ON teams;
DROP INDEX IF EXISTS teams_slug_idx;
DROP INDEX IF EXISTS teams_parent_team_id_idx;
DROP INDEX IF EXISTS teams_organization_id_idx;
DROP TABLE IF EXISTS teams;

DROP TRIGGER IF EXISTS on_users_update ON users;
DROP INDEX IF EXISTS users_email_idx;
DROP INDEX IF EXISTS users_role_idx;
DROP INDEX IF EXISTS users_kratos_id_idx;
DROP INDEX IF EXISTS users_username_trgm_idx;
DROP TABLE IF EXISTS users;

DROP TRIGGER IF EXISTS on_organizations_update ON organizations;
DROP INDEX IF EXISTS organizations_created_at_idx;
DROP INDEX IF EXISTS organizations_login_idx;
DROP TABLE IF EXISTS organizations;

-- Drop enums
DROP TYPE IF EXISTS organization_role;
DROP TYPE IF EXISTS team_role;
DROP TYPE IF EXISTS outbox_event_status;
DROP TYPE IF EXISTS package_status;
DROP TYPE IF EXISTS problem_permission;
DROP TYPE IF EXISTS problem_role;
DROP TYPE IF EXISTS contest_role;
DROP TYPE IF EXISTS contest_visibility;
DROP TYPE IF EXISTS problem_visibility;
DROP TYPE IF EXISTS user_role;

-- Drop functions
DROP FUNCTION IF EXISTS get_contest_scoreboard(UUID, BOOLEAN);
DROP FUNCTION IF EXISTS user_is_contest_moderator(UUID, UUID);
DROP FUNCTION IF EXISTS user_has_contest_access(UUID, UUID);
DROP FUNCTION IF EXISTS user_has_problem_access(UUID, UUID);
DROP FUNCTION IF EXISTS check_max_problems_on_contest();
DROP FUNCTION IF EXISTS updated_at_update();
DROP FUNCTION IF EXISTS uuid_generate_v7();

-- Drop extensions
DROP EXTENSION IF EXISTS pg_trgm;
DROP EXTENSION IF EXISTS "uuid-ossp";

-- +goose StatementEnd
