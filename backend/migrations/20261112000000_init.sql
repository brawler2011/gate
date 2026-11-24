-- +goose Up


CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;

CREATE TYPE auth_token_type AS ENUM (
    'email_verification',
    'password_reset',
    'email_change'
);

CREATE TYPE contest_scope AS ENUM (
    'public',
    'participant',
    'moderator'
);

-- Role enums ascend so max(role) picks the strongest grant.
CREATE TYPE contest_role AS ENUM (
    'participant',
    'moderator',
    'owner'
);

CREATE TYPE contest_visibility AS ENUM (
    'private',
    'public'
);

-- What authors see about their own submissions; others never see more.
CREATE TYPE feedback_level AS ENUM (
    'hidden',
    'verdict',
    'samples',
    'tests',
    'full'
);

CREATE TYPE freeze_status AS ENUM (
    'auto',
    'frozen',
    'unfrozen'
);

CREATE TYPE join_policy AS ENUM (
    'open',
    'by_request',
    'invite_only'
);

CREATE TYPE organization_role AS ENUM (
    'member',
    'maintainer',
    'admin',
    'owner'
);

CREATE TYPE package_status AS ENUM (
    'pending',
    'building',
    'ready',
    'failed'
);

CREATE TYPE problem_role AS ENUM (
    'viewer',
    'editor',
    'admin',
    'owner'
);

CREATE TYPE problem_visibility AS ENUM (
    'private',
    'public'
);

-- Only contest submissions count toward the standings.
CREATE TYPE submission_kind AS ENUM (
    'contest',
    'upsolving',
    'virtual'
);

CREATE TYPE team_role AS ENUM (
    'member',
    'maintainer'
);

CREATE TYPE user_role AS ENUM (
    'user',
    'admin'
);

-- +goose StatementBegin
CREATE FUNCTION updated_at_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;
-- +goose StatementEnd


CREATE TABLE users (
    id uuid NOT NULL,
    username varchar(70) NOT NULL,
    role user_role DEFAULT 'user'::user_role NOT NULL,
    email varchar(255),
    avatar_url text,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    password_hash text NOT NULL,
    expires_at timestamptz,
    claimed_by_user_id uuid,
    claimed_at timestamptz,
    is_email_verified boolean DEFAULT false NOT NULL,
    CONSTRAINT users_username_check CHECK ((length((username)::text) > 0)),
    CONSTRAINT users_pkey PRIMARY KEY (id),
    CONSTRAINT users_claimed_by_user_id_fkey FOREIGN KEY (claimed_by_user_id) REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX users_claimed_by_user_id_idx ON users USING btree (claimed_by_user_id);
CREATE UNIQUE INDEX users_email_idx ON users USING btree (email) WHERE (email IS NOT NULL);
CREATE INDEX users_expires_at_idx ON users USING btree (expires_at);
CREATE INDEX users_role_idx ON users USING btree (role);
CREATE UNIQUE INDEX users_username_lower_idx ON users USING btree (lower((username)::text));
CREATE INDEX users_username_trgm_idx ON users USING gin (lower((username)::text) gin_trgm_ops);

CREATE TABLE sessions (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT sessions_pkey PRIMARY KEY (id),
    CONSTRAINT sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX sessions_user_id_idx ON sessions USING btree (user_id);

CREATE TABLE rate_limits (
    key text NOT NULL,
    hits timestamptz[] DEFAULT '{}' NOT NULL,
    blocked_until timestamptz,
    expires_at timestamptz NOT NULL,
    CONSTRAINT rate_limits_pkey PRIMARY KEY (key)
);
CREATE INDEX rate_limits_expires_at_idx ON rate_limits USING btree (expires_at);

CREATE TABLE auth_tokens (
    id uuid DEFAULT uuidv7() NOT NULL,
    user_id uuid NOT NULL,
    token_type auth_token_type NOT NULL,
    token_hash text NOT NULL,
    payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT auth_tokens_pkey PRIMARY KEY (id),
    CONSTRAINT auth_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX auth_tokens_expires_at_idx ON auth_tokens USING btree (expires_at);
CREATE INDEX auth_tokens_token_hash_idx ON auth_tokens USING btree (token_hash);
CREATE INDEX auth_tokens_user_id_type_idx ON auth_tokens USING btree (user_id, token_type);

CREATE TRIGGER on_users_update BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION updated_at_update();


CREATE TABLE organizations (
    id uuid DEFAULT uuidv7() NOT NULL,
    login text NOT NULL,
    name text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    avatar_url text,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    join_policy join_policy DEFAULT 'by_request'::join_policy NOT NULL,
    -- IANA name; booklets print contest dates in it.
    time_zone text DEFAULT 'UTC'::text NOT NULL,
    CONSTRAINT organizations_login_check CHECK ((length(login) > 0)),
    CONSTRAINT organizations_name_check CHECK ((length(name) > 0)),
    CONSTRAINT organizations_pkey PRIMARY KEY (id)
);
CREATE INDEX organizations_created_at_idx ON organizations USING btree (created_at DESC);
CREATE UNIQUE INDEX organizations_login_lower_idx ON organizations USING btree (lower(login));
CREATE INDEX organizations_name_trgm_idx ON organizations USING gin (name gin_trgm_ops);

CREATE TABLE organization_members (
    organization_id uuid NOT NULL,
    user_id uuid NOT NULL,
    role organization_role DEFAULT 'member'::organization_role NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT organization_members_pkey PRIMARY KEY (organization_id, user_id),
    CONSTRAINT organization_members_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
    CONSTRAINT organization_members_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX organization_members_user_id_idx ON organization_members USING btree (user_id);

CREATE TABLE organization_join_requests (
    id uuid DEFAULT uuidv7() NOT NULL,
    organization_id uuid NOT NULL,
    user_id uuid NOT NULL,
    message text,
    status text DEFAULT 'pending'::text NOT NULL,
    reviewed_by uuid,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT organization_join_requests_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'approved'::text, 'rejected'::text, 'canceled'::text]))),
    CONSTRAINT organization_join_requests_pkey PRIMARY KEY (id),
    CONSTRAINT organization_join_requests_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
    CONSTRAINT organization_join_requests_reviewed_by_fkey FOREIGN KEY (reviewed_by) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT organization_join_requests_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX org_join_requests_org_idx ON organization_join_requests USING btree (organization_id, status);
CREATE UNIQUE INDEX org_join_requests_pending_unique_idx ON organization_join_requests USING btree (organization_id, user_id) WHERE (status = 'pending'::text);
CREATE INDEX org_join_requests_user_idx ON organization_join_requests USING btree (user_id, status);

CREATE TRIGGER on_organizations_update BEFORE UPDATE ON organizations FOR EACH ROW EXECUTE FUNCTION updated_at_update();


CREATE TABLE teams (
    id uuid DEFAULT uuidv7() NOT NULL,
    organization_id uuid NOT NULL,
    name text NOT NULL,
    slug text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT teams_name_check CHECK ((length(name) > 0)),
    CONSTRAINT teams_slug_check CHECK ((length(slug) > 0)),
    CONSTRAINT teams_organization_id_slug_key UNIQUE (organization_id, slug),
    CONSTRAINT teams_id_organization_unique UNIQUE (id, organization_id),
    CONSTRAINT teams_pkey PRIMARY KEY (id),
    CONSTRAINT teams_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE
);
CREATE INDEX teams_name_trgm_idx ON teams USING gin (name gin_trgm_ops);

CREATE TABLE team_members (
    team_id uuid NOT NULL,
    user_id uuid NOT NULL,
    role team_role DEFAULT 'member'::team_role NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT team_members_pkey PRIMARY KEY (team_id, user_id),
    CONSTRAINT team_members_team_id_fkey FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE,
    CONSTRAINT team_members_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX team_members_user_id_idx ON team_members USING btree (user_id);

CREATE TRIGGER on_teams_update BEFORE UPDATE ON teams FOR EACH ROW EXECUTE FUNCTION updated_at_update();


CREATE TABLE stored_objects (
    id uuid NOT NULL,
    organization_id uuid NOT NULL,
    purpose text NOT NULL,
    key text NOT NULL,
    sha256 text NOT NULL,
    size bigint NOT NULL,
    created_at timestamptz DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT stored_objects_check CHECK ((key = ((('objects/'::text || (organization_id)::text) || '/'::text) || (id)::text))),
    CONSTRAINT stored_objects_purpose_check CHECK ((purpose = ANY (ARRAY['test-input'::text, 'test-answer'::text, 'checker-test'::text, 'validator-test'::text, 'asset'::text, 'package'::text, 'statement'::text, 'booklet'::text, 'inputs'::text]))),
    CONSTRAINT stored_objects_sha256_check CHECK ((sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT stored_objects_size_check CHECK ((size >= 0)),
    CONSTRAINT stored_objects_key_key UNIQUE (key),
    CONSTRAINT stored_objects_pkey PRIMARY KEY (id),
    CONSTRAINT stored_objects_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE RESTRICT
);

CREATE TABLE problems (
    id uuid DEFAULT uuidv7() NOT NULL,
    organization_id uuid NOT NULL,
    visibility problem_visibility DEFAULT 'private'::problem_visibility NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    is_template boolean DEFAULT false NOT NULL,
    CONSTRAINT problems_id_organization_unique UNIQUE (id, organization_id),
    CONSTRAINT problems_pkey PRIMARY KEY (id),
    CONSTRAINT problems_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE
);
CREATE INDEX problems_created_at_idx ON problems USING btree (created_at DESC);
CREATE INDEX problems_organization_id_idx ON problems USING btree (organization_id);
CREATE INDEX problems_visibility_idx ON problems USING btree (visibility);

CREATE TABLE problem_members (
    problem_id uuid NOT NULL,
    user_id uuid NOT NULL,
    role problem_role DEFAULT 'viewer'::problem_role NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT problem_members_pkey PRIMARY KEY (problem_id, user_id),
    CONSTRAINT problem_members_problem_id_fkey FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE,
    CONSTRAINT problem_members_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX problem_members_user_id_idx ON problem_members USING btree (user_id);


CREATE TRIGGER on_problems_update BEFORE UPDATE ON problems FOR EACH ROW EXECUTE FUNCTION updated_at_update();


CREATE TABLE problem_drafts (
    problem_id uuid NOT NULL,
    organization_id uuid NOT NULL,
    revision bigint DEFAULT 1 NOT NULL,
    updated_at timestamptz DEFAULT clock_timestamp() NOT NULL,
    title text NOT NULL,
    short_name text NOT NULL,
    type text NOT NULL,
    time_limit_ms integer NOT NULL,
    memory_limit_mb integer NOT NULL,
    max_score numeric(12,2),
    -- The draft is validated and written as one document.
    content jsonb DEFAULT '{}'::jsonb NOT NULL,
    -- Stored objects the content references; written with it so GC need not parse drafts.
    object_ids uuid[] DEFAULT '{}'::uuid[] NOT NULL,
    CONSTRAINT problem_drafts_content_check CHECK ((jsonb_typeof(content) = 'object'::text)),
    CONSTRAINT problem_drafts_check CHECK (((type = ANY (ARRAY['scoring'::text, 'output-only'::text])) = (max_score IS NOT NULL))),
    CONSTRAINT problem_drafts_max_score_check CHECK ((max_score >= (0)::numeric)),
    CONSTRAINT problem_drafts_memory_limit_mb_check CHECK ((memory_limit_mb > 0)),
    CONSTRAINT problem_drafts_revision_check CHECK ((revision > 0)),
    CONSTRAINT problem_drafts_short_name_check CHECK (((octet_length(short_name) >= 1) AND (octet_length(short_name) <= 128))),
    CONSTRAINT problem_drafts_time_limit_ms_check CHECK ((time_limit_ms > 0)),
    CONSTRAINT problem_drafts_title_check CHECK ((octet_length(title) <= 1024)),
    CONSTRAINT problem_drafts_type_check CHECK ((type = ANY (ARRAY['pass-fail'::text, 'scoring'::text, 'interactive'::text, 'output-only'::text]))),
    CONSTRAINT problem_drafts_organization_id_short_name_key UNIQUE (organization_id, short_name),
    CONSTRAINT problem_drafts_pkey PRIMARY KEY (problem_id),
    CONSTRAINT problem_drafts_problem_id_fkey FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE,
    CONSTRAINT problem_drafts_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE
);
CREATE INDEX problem_drafts_title_trgm ON problem_drafts USING gin (title gin_trgm_ops);

CREATE VIEW problem_catalog AS
 SELECT p.id,
    p.organization_id,
    p.visibility,
    p.is_template,
    p.created_at,
    d.updated_at,
    d.title,
    d.short_name,
    d.time_limit_ms,
    d.memory_limit_mb
   FROM (problems p
     JOIN problem_drafts d ON ((d.problem_id = p.id)));


CREATE TABLE problem_builds (
    id uuid NOT NULL,
    problem_id uuid NOT NULL,
    organization_id uuid NOT NULL,
    revision bigint NOT NULL,
    snapshot jsonb NOT NULL,
    state text NOT NULL,
    lease_token uuid,
    lease_deadline timestamptz,
    attempt integer DEFAULT 0 NOT NULL,
    max_attempts integer NOT NULL,
    deadline timestamptz NOT NULL,
    diagnostic jsonb,
    -- What each solution that runs got; written before the build passes or fails on them.
    solutions jsonb,
    -- What each checker and validator test got, written like solutions.
    program_tests jsonb,
    created_at timestamptz DEFAULT clock_timestamp() NOT NULL,
    finished_at timestamptz,
    CONSTRAINT problem_builds_attempt_check CHECK ((attempt >= 0)),
    CONSTRAINT problem_builds_check CHECK (((state = 'running'::text) = ((lease_token IS NOT NULL) AND (lease_deadline IS NOT NULL)))),
    CONSTRAINT problem_builds_check1 CHECK (((state = ANY (ARRAY['succeeded'::text, 'failed'::text, 'cancelled'::text])) = (finished_at IS NOT NULL))),
    CONSTRAINT problem_builds_diagnostic_check CHECK ((octet_length((diagnostic)::text) <= 131072)),
    CONSTRAINT problem_builds_max_attempts_check CHECK ((max_attempts > 0)),
    CONSTRAINT problem_builds_revision_check CHECK ((revision > 0)),
    CONSTRAINT problem_builds_snapshot_check CHECK ((jsonb_typeof(snapshot) = 'object'::text)),
    CONSTRAINT problem_builds_solutions_check CHECK ((jsonb_typeof(solutions) = 'array'::text)),
    CONSTRAINT problem_builds_program_tests_check CHECK ((jsonb_typeof(program_tests) = 'array'::text)),
    CONSTRAINT problem_builds_state_check CHECK ((state = ANY (ARRAY['queued'::text, 'running'::text, 'succeeded'::text, 'failed'::text, 'cancelled'::text]))),
    CONSTRAINT problem_builds_pkey PRIMARY KEY (id),
    CONSTRAINT problem_builds_problem_id_fkey FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE RESTRICT,
    CONSTRAINT problem_builds_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE RESTRICT
);
CREATE INDEX problem_builds_queue ON problem_builds USING btree (created_at, id) WHERE (state = 'queued'::text);
CREATE INDEX problem_builds_recovery ON problem_builds USING btree (lease_deadline) WHERE (state = 'running'::text);

CREATE TABLE problem_packages (
    id uuid DEFAULT uuidv7() NOT NULL,
    problem_id uuid NOT NULL,
    organization_id uuid NOT NULL,
    package_hash varchar(64) NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    -- The index below is written once at publication so reads need not open the archive.
    title text NOT NULL,
    short_name text NOT NULL,
    type text NOT NULL,
    time_limit_ms integer NOT NULL,
    memory_limit_mb integer NOT NULL,
    input_file text NOT NULL,
    output_file text NOT NULL,
    build_id uuid NOT NULL,
    object_id uuid NOT NULL,
    revision bigint NOT NULL,
    samples jsonb NOT NULL,
    statements jsonb NOT NULL,
    -- The zip of every test input that participants of an output-only problem download.
    inputs_object_id uuid,
    CONSTRAINT problem_packages_inputs_check CHECK (((inputs_object_id IS NOT NULL) = (type = 'output-only'::text))),
    CONSTRAINT problem_packages_package_hash_check CHECK ((length((package_hash)::text) = 64)),
    CONSTRAINT problem_packages_revision_check CHECK ((revision > 0)),
    CONSTRAINT problem_packages_samples_check CHECK ((jsonb_typeof(samples) = 'array'::text)),
    CONSTRAINT problem_packages_statements_check CHECK ((jsonb_typeof(statements) = 'array'::text)),
    CONSTRAINT problem_packages_type_check CHECK ((type = ANY (ARRAY['pass-fail'::text, 'scoring'::text, 'interactive'::text, 'output-only'::text]))),
    CONSTRAINT problem_packages_build_id_key UNIQUE (build_id),
    CONSTRAINT problem_packages_pkey PRIMARY KEY (id),
    CONSTRAINT problem_packages_build_id_fkey FOREIGN KEY (build_id) REFERENCES problem_builds(id) ON DELETE RESTRICT,
    CONSTRAINT problem_packages_object_id_fkey FOREIGN KEY (object_id) REFERENCES stored_objects(id) ON DELETE RESTRICT,
    CONSTRAINT problem_packages_inputs_object_id_fkey FOREIGN KEY (inputs_object_id) REFERENCES stored_objects(id) ON DELETE RESTRICT,
    CONSTRAINT problem_packages_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
    CONSTRAINT problem_packages_problem_id_fkey FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE
);
CREATE INDEX problem_packages_created_at_idx ON problem_packages USING btree (created_at DESC);
CREATE INDEX problem_packages_object ON problem_packages USING btree (object_id);
CREATE INDEX problem_packages_inputs_object ON problem_packages USING btree (inputs_object_id);
CREATE INDEX problem_packages_organization_id_idx ON problem_packages USING btree (organization_id);
CREATE INDEX problem_packages_package_hash_idx ON problem_packages USING btree (package_hash);
CREATE INDEX problem_packages_problem_id_idx ON problem_packages USING btree (problem_id);

CREATE TABLE problem_package_assets (
    package_id uuid NOT NULL,
    name text NOT NULL,
    object_id uuid NOT NULL,
    CONSTRAINT problem_package_assets_pkey PRIMARY KEY (package_id, name),
    CONSTRAINT problem_package_assets_object_id_fkey FOREIGN KEY (object_id) REFERENCES stored_objects(id) ON DELETE RESTRICT,
    CONSTRAINT problem_package_assets_package_id_fkey FOREIGN KEY (package_id) REFERENCES problem_packages(id) ON DELETE CASCADE
);
CREATE INDEX problem_package_assets_object ON problem_package_assets USING btree (object_id);

-- The PDF of each statement language a build typeset.
CREATE TABLE problem_package_pdfs (
    package_id uuid NOT NULL,
    language text NOT NULL,
    object_id uuid NOT NULL,
    CONSTRAINT problem_package_pdfs_pkey PRIMARY KEY (package_id, language),
    CONSTRAINT problem_package_pdfs_object_id_fkey FOREIGN KEY (object_id) REFERENCES stored_objects(id) ON DELETE RESTRICT,
    CONSTRAINT problem_package_pdfs_package_id_fkey FOREIGN KEY (package_id) REFERENCES problem_packages(id) ON DELETE CASCADE
);
CREATE INDEX problem_package_pdfs_object ON problem_package_pdfs USING btree (object_id);


CREATE TABLE contests (
    id uuid DEFAULT uuidv7() NOT NULL,
    organization_id uuid NOT NULL,
    visibility contest_visibility DEFAULT 'private'::contest_visibility NOT NULL,
    title text NOT NULL,
    -- Printed in booklet headers; empty prints the organization name.
    place text DEFAULT ''::text NOT NULL,
    login varchar(64) NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    join_policy join_policy DEFAULT 'open'::join_policy NOT NULL,
    -- Audiences of other people's results while the contest runs and after it ends.
    scoreboard_during contest_scope DEFAULT 'participant'::contest_scope NOT NULL,
    scoreboard_after contest_scope DEFAULT 'public'::contest_scope NOT NULL,
    submissions_during contest_scope DEFAULT 'moderator'::contest_scope NOT NULL,
    submissions_after contest_scope DEFAULT 'participant'::contest_scope NOT NULL,
    sources_during contest_scope DEFAULT 'moderator'::contest_scope NOT NULL,
    sources_after contest_scope DEFAULT 'moderator'::contest_scope NOT NULL,
    reports_during contest_scope DEFAULT 'moderator'::contest_scope NOT NULL,
    reports_after contest_scope DEFAULT 'moderator'::contest_scope NOT NULL,
    tutorials_during contest_scope DEFAULT 'moderator'::contest_scope NOT NULL,
    tutorials_after contest_scope DEFAULT 'participant'::contest_scope NOT NULL,
    feedback_during feedback_level DEFAULT 'verdict'::feedback_level NOT NULL,
    feedback_after feedback_level DEFAULT 'full'::feedback_level NOT NULL,
    freeze_status freeze_status DEFAULT 'auto'::freeze_status NOT NULL,
    freeze_duration_minutes integer,
    freeze_started_at timestamptz,
    freeze_hides_attempts boolean DEFAULT false NOT NULL,
    -- NULL lifts the freeze at the end.
    unfreeze_time timestamptz,
    hide_statements boolean DEFAULT false NOT NULL,
    allow_clarifications boolean DEFAULT true NOT NULL,
    enable_drafts boolean DEFAULT true NOT NULL,
    enable_upsolving boolean DEFAULT true NOT NULL,
    allow_virtual boolean DEFAULT false NOT NULL,
    -- Contest submissions are judged on pretests until the end, then on all tests.
    pretests boolean DEFAULT false NOT NULL,
    languages text[] DEFAULT '{}'::text[] NOT NULL,
    start_time timestamptz,
    end_time timestamptz,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT contests_check CHECK (((end_time IS NULL) OR (start_time IS NULL) OR (end_time > start_time))),
    CONSTRAINT contests_freeze_duration_check CHECK ((freeze_duration_minutes > 0)),
    CONSTRAINT contests_freeze_started_check CHECK (((freeze_status = 'frozen'::freeze_status) = (freeze_started_at IS NOT NULL))),
    CONSTRAINT contests_unfreeze_time_check CHECK ((unfreeze_time >= end_time)),
    CONSTRAINT contests_pretests_check CHECK (((NOT pretests) OR (end_time IS NOT NULL))),
    CONSTRAINT contests_login_check CHECK (((length((login)::text) >= 3) AND (length((login)::text) <= 64))),
    CONSTRAINT contests_pkey PRIMARY KEY (id),
    CONSTRAINT contests_id_organization_unique UNIQUE (id, organization_id),
    CONSTRAINT contests_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE
);
CREATE INDEX contests_created_at_idx ON contests USING btree (created_at DESC);
CREATE UNIQUE INDEX contests_org_id_login_lower_idx ON contests USING btree (organization_id, lower((login)::text));
CREATE INDEX contests_organization_id_idx ON contests USING btree (organization_id);
CREATE INDEX contests_start_time_idx ON contests USING btree (start_time);
CREATE INDEX contests_title_trgm_idx ON contests USING gin (title gin_trgm_ops);
CREATE INDEX contests_visibility_idx ON contests USING btree (visibility);

CREATE TABLE contest_members (
    contest_id uuid NOT NULL,
    user_id uuid NOT NULL,
    role contest_role DEFAULT 'participant'::contest_role NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT contest_members_pkey PRIMARY KEY (contest_id, user_id),
    CONSTRAINT contest_members_contest_id_fkey FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    CONSTRAINT contest_members_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX contest_members_user_id_idx ON contest_members USING btree (user_id);

CREATE TABLE contest_teams (
    contest_id uuid NOT NULL,
    team_id uuid NOT NULL,
    organization_id uuid NOT NULL,
    role contest_role DEFAULT 'participant'::contest_role NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT contest_teams_role_check CHECK ((role <> 'owner'::contest_role)),
    CONSTRAINT contest_teams_pkey PRIMARY KEY (contest_id, team_id),
    CONSTRAINT contest_teams_contest_id_fkey FOREIGN KEY (contest_id, organization_id) REFERENCES contests(id, organization_id) ON DELETE CASCADE,
    CONSTRAINT contest_teams_team_id_fkey FOREIGN KEY (team_id, organization_id) REFERENCES teams(id, organization_id) ON DELETE CASCADE
);
CREATE INDEX contest_teams_team_id_idx ON contest_teams USING btree (team_id);

CREATE TABLE contest_problems (
    contest_id uuid NOT NULL,
    problem_id uuid NOT NULL,
    package_id uuid NOT NULL,
    ordinal integer NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT contest_problems_ordinal_check CHECK ((ordinal >= 0)),
    CONSTRAINT contest_problems_contest_id_ordinal_key UNIQUE (contest_id, ordinal),
    CONSTRAINT contest_problems_pkey PRIMARY KEY (contest_id, problem_id),
    CONSTRAINT contest_problems_contest_id_fkey FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    CONSTRAINT contest_problems_package_id_fkey FOREIGN KEY (package_id) REFERENCES problem_packages(id) ON DELETE RESTRICT,
    CONSTRAINT contest_problems_problem_id_fkey FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE RESTRICT
);
CREATE INDEX contest_problems_package_id_idx ON contest_problems USING btree (package_id);
CREATE INDEX contest_problems_problem_id_idx ON contest_problems USING btree (problem_id);

-- One booklet per contest and statement language; key hashes everything it prints, so a booklet is current iff its key matches.
CREATE TABLE contest_booklets (
    contest_id uuid NOT NULL,
    organization_id uuid NOT NULL,
    language text NOT NULL,
    key text NOT NULL,
    state text DEFAULT 'queued'::text NOT NULL,
    attempt integer DEFAULT 0 NOT NULL,
    lease_token uuid,
    lease_deadline timestamptz,
    object_id uuid,
    diagnostic text,
    updated_at timestamptz DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT contest_booklets_state_check CHECK ((state = ANY (ARRAY['queued'::text, 'running'::text, 'succeeded'::text, 'failed'::text]))),
    CONSTRAINT contest_booklets_lease_check CHECK (((state = 'running'::text) = ((lease_token IS NOT NULL) AND (lease_deadline IS NOT NULL)))),
    CONSTRAINT contest_booklets_object_check CHECK (((state = 'succeeded'::text) = (object_id IS NOT NULL))),
    CONSTRAINT contest_booklets_diagnostic_check CHECK (((state = 'failed'::text) = (diagnostic IS NOT NULL))),
    CONSTRAINT contest_booklets_key_check CHECK ((key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT contest_booklets_pkey PRIMARY KEY (contest_id, language),
    CONSTRAINT contest_booklets_contest_id_fkey FOREIGN KEY (contest_id, organization_id) REFERENCES contests(id, organization_id) ON DELETE CASCADE,
    CONSTRAINT contest_booklets_object_id_fkey FOREIGN KEY (object_id) REFERENCES stored_objects(id) ON DELETE RESTRICT
);
CREATE INDEX contest_booklets_queue ON contest_booklets USING btree (updated_at) WHERE (state = ANY (ARRAY['queued'::text, 'running'::text]));
CREATE INDEX contest_booklets_object ON contest_booklets USING btree (object_id);

CREATE TABLE contest_user_problem_blocks (
    contest_id uuid NOT NULL,
    user_id uuid NOT NULL,
    problem_id uuid NOT NULL,
    reason text,
    created_by uuid,
    created_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT contest_user_problem_blocks_pkey PRIMARY KEY (contest_id, user_id, problem_id),
    CONSTRAINT contest_user_problem_blocks_contest_id_fkey FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    CONSTRAINT contest_user_problem_blocks_created_by_fkey FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT contest_user_problem_blocks_problem_id_fkey FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE,
    CONSTRAINT contest_user_problem_blocks_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX contest_user_problem_blocks_contest_idx ON contest_user_problem_blocks USING btree (contest_id);

CREATE TABLE contest_join_requests (
    id uuid DEFAULT uuidv7() NOT NULL,
    contest_id uuid NOT NULL,
    user_id uuid NOT NULL,
    message text,
    status text DEFAULT 'pending'::text NOT NULL,
    reviewed_by uuid,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT contest_join_requests_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'approved'::text, 'rejected'::text, 'canceled'::text]))),
    CONSTRAINT contest_join_requests_pkey PRIMARY KEY (id),
    CONSTRAINT contest_join_requests_contest_id_fkey FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    CONSTRAINT contest_join_requests_reviewed_by_fkey FOREIGN KEY (reviewed_by) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT contest_join_requests_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX contest_join_requests_contest_idx ON contest_join_requests USING btree (contest_id, status);
CREATE UNIQUE INDEX contest_join_requests_pending_unique_idx ON contest_join_requests USING btree (contest_id, user_id) WHERE (status = 'pending'::text);
CREATE INDEX contest_join_requests_user_idx ON contest_join_requests USING btree (user_id, status);

-- A named invitation (user_id) is accepted or declined once; a join link (token) has no user and is reusable.
CREATE TABLE invitations (
    id uuid DEFAULT uuidv7() NOT NULL,
    organization_id uuid,
    team_id uuid,
    contest_id uuid,
    role text NOT NULL,
    user_id uuid,
    token text,
    inviter_id uuid NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT invitations_target_check CHECK ((num_nonnulls(organization_id, team_id, contest_id) = 1)),
    CONSTRAINT invitations_kind_check CHECK ((num_nonnulls(user_id, token) = 1)),
    CONSTRAINT invitations_pkey PRIMARY KEY (id),
    CONSTRAINT invitations_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
    CONSTRAINT invitations_team_id_fkey FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE,
    CONSTRAINT invitations_contest_id_fkey FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    CONSTRAINT invitations_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT invitations_inviter_id_fkey FOREIGN KEY (inviter_id) REFERENCES users(id) ON DELETE CASCADE
);
-- One pending invitation per user and one link per target.
CREATE UNIQUE INDEX invitations_target_user_idx ON invitations USING btree (organization_id, team_id, contest_id, user_id) NULLS NOT DISTINCT;
CREATE UNIQUE INDEX invitations_token_idx ON invitations USING btree (token);
CREATE INDEX invitations_user_id_idx ON invitations USING btree (user_id);
CREATE INDEX invitations_team_id_idx ON invitations USING btree (team_id);
CREATE INDEX invitations_contest_id_idx ON invitations USING btree (contest_id);

CREATE TABLE contest_announcements (
    id uuid DEFAULT uuidv7() NOT NULL,
    contest_id uuid NOT NULL,
    problem_id uuid,
    author_id uuid NOT NULL,
    title text NOT NULL,
    body text NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT contest_announcements_pkey PRIMARY KEY (id),
    CONSTRAINT contest_announcements_author_id_fkey FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT contest_announcements_contest_id_fkey FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    CONSTRAINT contest_announcements_problem_id_fkey FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE
);
CREATE INDEX contest_announcements_contest_idx ON contest_announcements USING btree (contest_id, created_at DESC);

CREATE TABLE contest_clarifications (
    id uuid DEFAULT uuidv7() NOT NULL,
    contest_id uuid NOT NULL,
    problem_id uuid,
    user_id uuid NOT NULL,
    question text NOT NULL,
    answer text,
    answered_by uuid,
    status text DEFAULT 'pending'::text NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    answered_at timestamptz,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT contest_clarifications_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'answered'::text]))),
    CONSTRAINT contest_clarifications_pkey PRIMARY KEY (id),
    CONSTRAINT contest_clarifications_answered_by_fkey FOREIGN KEY (answered_by) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT contest_clarifications_contest_id_fkey FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    CONSTRAINT contest_clarifications_problem_id_fkey FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE,
    CONSTRAINT contest_clarifications_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX contest_clarifications_contest_status_idx ON contest_clarifications USING btree (contest_id, status, created_at DESC);
CREATE INDEX contest_clarifications_contest_user_idx ON contest_clarifications USING btree (contest_id, user_id, created_at DESC);

CREATE TABLE contest_drafts (
    id uuid DEFAULT uuidv7() NOT NULL,
    contest_id uuid NOT NULL,
    user_id uuid NOT NULL,
    code text NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT contest_drafts_pkey PRIMARY KEY (id),
    CONSTRAINT contest_drafts_contest_id_fkey FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    CONSTRAINT contest_drafts_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX contest_drafts_contest_user_idx ON contest_drafts USING btree (contest_id, user_id, created_at DESC);

-- A personal replay of a finished contest; the window is fixed at start so later edits of the contest times keep the run.
CREATE TABLE contest_virtual_runs (
    contest_id uuid NOT NULL,
    user_id uuid NOT NULL,
    started_at timestamptz NOT NULL,
    ends_at timestamptz NOT NULL,
    finished_at timestamptz,
    CONSTRAINT contest_virtual_runs_check CHECK ((ends_at > started_at)),
    CONSTRAINT contest_virtual_runs_finished_check CHECK ((finished_at >= started_at AND finished_at < ends_at)),
    CONSTRAINT contest_virtual_runs_pkey PRIMARY KEY (contest_id, user_id),
    CONSTRAINT contest_virtual_runs_contest_id_fkey FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    CONSTRAINT contest_virtual_runs_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- +goose StatementBegin
CREATE FUNCTION capture_manual_freeze() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  IF NEW.freeze_status='frozen' THEN
    IF TG_OP='INSERT' OR OLD.freeze_status<>'frozen' THEN
      NEW.freeze_started_at=clock_timestamp();
    ELSE
      NEW.freeze_started_at=OLD.freeze_started_at;
    END IF;
  ELSE
    NEW.freeze_started_at=NULL;
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER manual_freeze_changed BEFORE INSERT OR UPDATE OF freeze_status, freeze_started_at ON contests FOR EACH ROW EXECUTE FUNCTION capture_manual_freeze();
CREATE TRIGGER on_contests_update BEFORE UPDATE ON contests FOR EACH ROW EXECUTE FUNCTION updated_at_update();


CREATE TABLE submissions (
    id uuid DEFAULT uuidv7() NOT NULL,
    contest_id uuid,
    problem_id uuid,
    owner_id uuid,
    kind submission_kind NOT NULL,
    source varchar(65536) NOT NULL,
    language text NOT NULL,
    state integer DEFAULT 1 NOT NULL,
    score numeric(12,2) DEFAULT 0 NOT NULL,
    penalty integer DEFAULT 0 NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    test_details jsonb,
    ban_reason text,
    revision bigint DEFAULT 1 NOT NULL,
    package_id uuid,
    -- Judged or queued on pretests only; system testing rejudges it after the contest ends.
    pretests boolean DEFAULT false NOT NULL,
    -- Strict mode: the time limit factor and the benchmark that measured it.
    judge_k double precision,
    judge_benchmark text,
    -- SHA-256 of the archive of an output-only submission's outputs, which replaces its source.
    outputs text,
    CONSTRAINT submissions_outputs_check CHECK ((outputs ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT submissions_calibration_check CHECK (((judge_k IS NULL) = (judge_benchmark IS NULL))),
    CONSTRAINT submissions_pkey PRIMARY KEY (id),
    CONSTRAINT submissions_contest_id_fkey FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE SET NULL,
    CONSTRAINT submissions_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT submissions_package_id_fkey FOREIGN KEY (package_id) REFERENCES problem_packages(id),
    CONSTRAINT submissions_problem_id_fkey FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE SET NULL
);
CREATE INDEX submissions_contest_problem_idx ON submissions USING btree (contest_id, problem_id);
CREATE INDEX submissions_created_at_idx ON submissions USING btree (created_at DESC, id DESC);
CREATE INDEX submissions_owner_id_idx ON submissions USING btree (owner_id);
CREATE INDEX submissions_problem_id_idx ON submissions USING btree (problem_id);
CREATE INDEX submissions_scoreboard_idx ON submissions USING btree (contest_id, owner_id, problem_id, created_at) WHERE ((contest_id IS NOT NULL) AND (owner_id IS NOT NULL));
CREATE INDEX submissions_scoreboard_state_idx ON submissions USING btree (contest_id, owner_id, state, created_at) WHERE ((contest_id IS NOT NULL) AND (owner_id IS NOT NULL));
CREATE INDEX submissions_state_idx ON submissions USING btree (state);
CREATE INDEX submissions_pretests_idx ON submissions USING btree (contest_id) WHERE pretests;

CREATE TABLE judge_jobs (
    id uuid NOT NULL,
    submission_id uuid NOT NULL,
    -- The submission revision the job judges; a later revision supersedes it.
    revision bigint NOT NULL,
    package_id uuid,
    package_hash text NOT NULL,
    state text DEFAULT 'pending'::text NOT NULL,
    attempt integer DEFAULT 0 NOT NULL,
    lease_token uuid,
    deadline_at timestamptz,
    created_at timestamptz DEFAULT now() NOT NULL,
    available_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT judge_jobs_attempt_check CHECK ((attempt >= 0)),
    CONSTRAINT judge_jobs_revision_check CHECK ((revision > 0)),
    CONSTRAINT judge_jobs_state_check CHECK ((state = ANY (ARRAY['pending'::text, 'processing'::text]))),
    CONSTRAINT judge_jobs_pkey PRIMARY KEY (id),
    CONSTRAINT judge_jobs_submission_id_revision_key UNIQUE (submission_id, revision),
    CONSTRAINT judge_jobs_package_id_fkey FOREIGN KEY (package_id) REFERENCES problem_packages(id) ON DELETE SET NULL,
    CONSTRAINT judge_jobs_submission_id_fkey FOREIGN KEY (submission_id) REFERENCES submissions(id) ON DELETE CASCADE
);
CREATE INDEX judge_jobs_recovery ON judge_jobs USING btree (deadline_at) WHERE (state = 'processing'::text);
CREATE INDEX judge_jobs_pending ON judge_jobs USING btree (created_at) WHERE (state = 'pending'::text);

CREATE TRIGGER on_submissions_update BEFORE UPDATE ON submissions FOR EACH ROW EXECUTE FUNCTION updated_at_update();




CREATE TABLE notifications (
    id uuid DEFAULT uuidv7() NOT NULL,
    user_id uuid NOT NULL,
    type varchar(64) NOT NULL,
    title text NOT NULL,
    body text NOT NULL,
    link text,
    data jsonb DEFAULT '{}'::jsonb NOT NULL,
    is_read boolean DEFAULT false NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT notifications_pkey PRIMARY KEY (id),
    CONSTRAINT notifications_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX notifications_user_created_idx ON notifications USING btree (user_id, created_at DESC);
CREATE INDEX notifications_user_unread_idx ON notifications USING btree (user_id, is_read, created_at DESC);

CREATE TABLE posts (
    id uuid DEFAULT uuidv7() NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    title text NOT NULL,
    text text NOT NULL,
    description text NOT NULL,
    author_id uuid NOT NULL,
    image_key text NOT NULL,
    CONSTRAINT posts_pkey PRIMARY KEY (id),
    CONSTRAINT posts_author_id_fkey FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_posts_author_id ON posts USING btree (author_id);
CREATE INDEX idx_posts_created_at ON posts USING btree (created_at DESC);

CREATE TRIGGER posts_updated_at_trigger BEFORE UPDATE ON posts FOR EACH ROW EXECUTE FUNCTION updated_at_update();

-- Organizations, problems and contests always keep at least one permanent owner.
-- Arguments: resource table, member table, member column referencing the resource.
-- +goose StatementBegin
CREATE FUNCTION check_owners() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    row_data record := CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
    resource uuid;
    missing boolean;
BEGIN
    IF TG_TABLE_NAME = TG_ARGV[0] THEN
        resource := row_data.id;
    ELSE
        EXECUTE format('SELECT ($1).%I', TG_ARGV[2]) INTO resource USING row_data;
    END IF;
    EXECUTE format(
        'SELECT EXISTS (SELECT 1 FROM %I WHERE id = $1) AND NOT EXISTS (
            SELECT 1 FROM %I m JOIN users u ON u.id = m.user_id
            WHERE m.%I = $1 AND m.role = ''owner'' AND u.expires_at IS NULL)',
        TG_ARGV[0], TG_ARGV[1], TG_ARGV[2]) INTO missing USING resource;
    IF missing THEN
        RAISE EXCEPTION '% % needs a permanent owner', TG_ARGV[0], resource
            USING ERRCODE = 'check_violation', CONSTRAINT = 'owner_required';
    END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER owner_required AFTER INSERT ON organizations DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_owners('organizations', 'organization_members', 'organization_id');
CREATE CONSTRAINT TRIGGER owner_required AFTER INSERT OR UPDATE OR DELETE ON organization_members DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_owners('organizations', 'organization_members', 'organization_id');
CREATE CONSTRAINT TRIGGER owner_required AFTER INSERT ON problems DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_owners('problems', 'problem_members', 'problem_id');
CREATE CONSTRAINT TRIGGER owner_required AFTER INSERT OR UPDATE OR DELETE ON problem_members DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_owners('problems', 'problem_members', 'problem_id');
CREATE CONSTRAINT TRIGGER owner_required AFTER INSERT ON contests DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_owners('contests', 'contest_members', 'contest_id');
CREATE CONSTRAINT TRIGGER owner_required AFTER INSERT OR UPDATE OR DELETE ON contest_members DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_owners('contests', 'contest_members', 'contest_id');

-- Effective roles: the strongest of a direct grant, a team grant and the organization role.
-- Site administrators and guests are handled in Go.
CREATE VIEW problem_access AS
SELECT problem_id, user_id, max(role) AS role FROM (
    SELECT problem_id, user_id, role FROM problem_members
    UNION ALL
    SELECT p.id, om.user_id, CASE om.role WHEN 'owner' THEN 'owner'::problem_role WHEN 'admin' THEN 'admin'::problem_role ELSE 'viewer'::problem_role END
    FROM problems p JOIN organization_members om ON om.organization_id = p.organization_id
    WHERE om.role IN ('maintainer', 'admin', 'owner')
) grants GROUP BY problem_id, user_id;

CREATE VIEW contest_access AS
SELECT contest_id, user_id, max(role) AS role FROM (
    SELECT contest_id, user_id, role FROM contest_members
    UNION ALL
    SELECT link.contest_id, tm.user_id, link.role
    FROM contest_teams link JOIN team_members tm ON tm.team_id = link.team_id
    UNION ALL
    SELECT c.id, om.user_id, CASE om.role WHEN 'owner' THEN 'owner'::contest_role ELSE 'moderator'::contest_role END
    FROM contests c JOIN organization_members om ON om.organization_id = c.organization_id
    WHERE om.role IN ('admin', 'owner')
) grants GROUP BY contest_id, user_id;


