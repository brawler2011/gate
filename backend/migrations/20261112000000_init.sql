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
    'admin',
    'owner'
);

CREATE TYPE participation_mode AS ENUM (
    'open',
    'by_request',
    'invite_only'
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

-- manager is only derived from organization admins; nobody holds it directly.
CREATE TYPE team_role AS ENUM (
    'member',
    'maintainer',
    'manager'
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
    id uuid DEFAULT gen_random_uuid() NOT NULL,
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

CREATE TABLE organization_invitations (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    organization_id uuid NOT NULL,
    user_id uuid NOT NULL,
    inviter_id uuid NOT NULL,
    role varchar(32) DEFAULT 'member'::varchar NOT NULL,
    status varchar(32) DEFAULT 'pending'::varchar NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT organization_invitations_pkey PRIMARY KEY (id),
    CONSTRAINT organization_invitations_inviter_id_fkey FOREIGN KEY (inviter_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT organization_invitations_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
    CONSTRAINT organization_invitations_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX org_invitations_org_idx ON organization_invitations USING btree (organization_id, status);
CREATE UNIQUE INDEX org_invitations_pending_unique_idx ON organization_invitations USING btree (organization_id, user_id) WHERE ((status)::text = 'pending'::text);
CREATE INDEX org_invitations_user_idx ON organization_invitations USING btree (user_id, status);

CREATE TABLE organization_join_requests (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    organization_id uuid NOT NULL,
    user_id uuid NOT NULL,
    message text,
    status varchar(32) DEFAULT 'pending'::varchar NOT NULL,
    reviewed_by uuid,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT organization_join_requests_pkey PRIMARY KEY (id),
    CONSTRAINT organization_join_requests_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
    CONSTRAINT organization_join_requests_reviewed_by_fkey FOREIGN KEY (reviewed_by) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT organization_join_requests_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX org_join_requests_org_idx ON organization_join_requests USING btree (organization_id, status);
CREATE UNIQUE INDEX org_join_requests_pending_unique_idx ON organization_join_requests USING btree (organization_id, user_id) WHERE ((status)::text = 'pending'::text);
CREATE INDEX org_join_requests_user_idx ON organization_join_requests USING btree (user_id, status);

CREATE TRIGGER on_organizations_update BEFORE UPDATE ON organizations FOR EACH ROW EXECUTE FUNCTION updated_at_update();


CREATE TABLE teams (
    id uuid DEFAULT uuidv7() NOT NULL,
    organization_id uuid NOT NULL,
    name text NOT NULL,
    slug text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    privacy text DEFAULT 'closed'::text NOT NULL,
    parent_team_id uuid,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT teams_name_check CHECK ((length(name) > 0)),
    CONSTRAINT teams_privacy_check CHECK ((privacy = ANY (ARRAY['secret'::text, 'closed'::text]))),
    CONSTRAINT teams_slug_check CHECK ((length(slug) > 0)),
    CONSTRAINT teams_organization_id_slug_key UNIQUE (organization_id, slug),
    CONSTRAINT teams_id_organization_unique UNIQUE (id, organization_id),
    CONSTRAINT teams_pkey PRIMARY KEY (id),
    CONSTRAINT teams_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
    CONSTRAINT teams_parent_team_id_fkey FOREIGN KEY (parent_team_id) REFERENCES teams(id) ON DELETE SET NULL
);
CREATE INDEX teams_name_trgm_idx ON teams USING gin (name gin_trgm_ops);
CREATE INDEX teams_organization_id_idx ON teams USING btree (organization_id);
CREATE INDEX teams_parent_team_id_idx ON teams USING btree (parent_team_id);
CREATE INDEX teams_slug_idx ON teams USING btree (organization_id, slug);

CREATE TABLE team_members (
    team_id uuid NOT NULL,
    user_id uuid NOT NULL,
    role team_role DEFAULT 'member'::team_role NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT team_members_role_check CHECK ((role <> 'manager'::team_role)),
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
    CONSTRAINT stored_objects_purpose_check CHECK ((purpose = ANY (ARRAY['test-input'::text, 'test-answer'::text, 'checker-test'::text, 'validator-test'::text, 'asset'::text, 'package'::text]))),
    CONSTRAINT stored_objects_sha256_check CHECK ((sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT stored_objects_size_check CHECK ((size >= 0)),
    CONSTRAINT stored_objects_id_organization_id_key UNIQUE (id, organization_id),
    CONSTRAINT stored_objects_key_key UNIQUE (key),
    CONSTRAINT stored_objects_pkey PRIMARY KEY (id),
    CONSTRAINT stored_objects_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE RESTRICT
);

CREATE TABLE request_idempotency (
    organization_id uuid NOT NULL,
    operation text NOT NULL,
    key text NOT NULL,
    fingerprint text NOT NULL,
    token uuid NOT NULL,
    lease_deadline timestamptz NOT NULL,
    result jsonb,
    http_status integer,
    created_at timestamptz DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT request_idempotency_check CHECK (((result IS NULL) = (http_status IS NULL))),
    CONSTRAINT request_idempotency_fingerprint_check CHECK ((fingerprint ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT request_idempotency_http_status_check CHECK (((http_status >= 200) AND (http_status <= 299))),
    CONSTRAINT request_idempotency_key_check CHECK (((octet_length(key) >= 1) AND (octet_length(key) <= 200))),
    CONSTRAINT request_idempotency_pkey PRIMARY KEY (organization_id, operation, key),
    CONSTRAINT request_idempotency_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE RESTRICT
);

-- Publish verified bytes with their first reference; object rows are immutable.
-- +goose StatementBegin
CREATE FUNCTION guard_stored_object() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN RAISE EXCEPTION 'stored object is immutable'; END;
$$;
-- +goose StatementEnd

CREATE TRIGGER stored_object_guard BEFORE UPDATE ON stored_objects FOR EACH ROW EXECUTE FUNCTION guard_stored_object();


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
CREATE INDEX problem_members_problem_id_idx ON problem_members USING btree (problem_id);
CREATE INDEX problem_members_user_id_idx ON problem_members USING btree (user_id);

-- A team grants access only to resources of its own organization.
CREATE TABLE problem_teams (
    problem_id uuid NOT NULL,
    team_id uuid NOT NULL,
    organization_id uuid NOT NULL,
    role problem_role DEFAULT 'viewer'::problem_role NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT problem_teams_role_check CHECK ((role <> 'owner'::problem_role)),
    CONSTRAINT problem_teams_pkey PRIMARY KEY (problem_id, team_id),
    CONSTRAINT problem_teams_problem_id_fkey FOREIGN KEY (problem_id, organization_id) REFERENCES problems(id, organization_id) ON DELETE CASCADE,
    CONSTRAINT problem_teams_team_id_fkey FOREIGN KEY (team_id, organization_id) REFERENCES teams(id, organization_id) ON DELETE CASCADE
);
CREATE INDEX problem_teams_problem_id_idx ON problem_teams USING btree (problem_id);
CREATE INDEX problem_teams_team_id_idx ON problem_teams USING btree (team_id);

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
    CONSTRAINT problem_drafts_content_check CHECK ((jsonb_typeof(content) = 'object'::text)),
    CONSTRAINT problem_drafts_check CHECK (((type = 'scoring'::text) = (max_score IS NOT NULL))),
    CONSTRAINT problem_drafts_max_score_check CHECK ((max_score >= (0)::numeric)),
    CONSTRAINT problem_drafts_memory_limit_mb_check CHECK ((memory_limit_mb > 0)),
    CONSTRAINT problem_drafts_revision_check CHECK ((revision > 0)),
    CONSTRAINT problem_drafts_short_name_check CHECK (((octet_length(short_name) >= 1) AND (octet_length(short_name) <= 128))),
    CONSTRAINT problem_drafts_time_limit_ms_check CHECK ((time_limit_ms > 0)),
    CONSTRAINT problem_drafts_title_check CHECK ((octet_length(title) <= 1024)),
    CONSTRAINT problem_drafts_type_check CHECK ((type = ANY (ARRAY['pass-fail'::text, 'scoring'::text, 'interactive'::text]))),
    CONSTRAINT problem_drafts_organization_id_short_name_key UNIQUE (organization_id, short_name),
    CONSTRAINT problem_drafts_pkey PRIMARY KEY (problem_id),
    CONSTRAINT problem_drafts_problem_id_organization_id_key UNIQUE (problem_id, organization_id),
    CONSTRAINT problem_drafts_problem_id_organization_id_fkey FOREIGN KEY (problem_id, organization_id) REFERENCES problems(id, organization_id) ON DELETE CASCADE
);
CREATE INDEX problem_drafts_title_trgm ON problem_drafts USING gin (title gin_trgm_ops);

-- Derived references keep every object in the draft alive.
CREATE TABLE problem_draft_objects (
    problem_id uuid NOT NULL,
    organization_id uuid NOT NULL,
    object_id uuid NOT NULL,
    CONSTRAINT problem_draft_objects_pkey PRIMARY KEY (problem_id, object_id),
    CONSTRAINT problem_draft_objects_problem_id_organization_id_fkey FOREIGN KEY (problem_id, organization_id) REFERENCES problem_drafts(problem_id, organization_id) ON DELETE CASCADE,
    CONSTRAINT problem_draft_objects_object_id_organization_id_fkey FOREIGN KEY (object_id, organization_id) REFERENCES stored_objects(id, organization_id) ON DELETE RESTRICT
);
CREATE INDEX problem_draft_objects_object ON problem_draft_objects USING btree (object_id);

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
    CONSTRAINT problem_builds_id_problem_id_organization_id_key UNIQUE (id, problem_id, organization_id),
    CONSTRAINT problem_builds_id_organization_id_key UNIQUE (id, organization_id),
    CONSTRAINT problem_builds_pkey PRIMARY KEY (id),
    CONSTRAINT problem_builds_problem_id_organization_id_fkey FOREIGN KEY (problem_id, organization_id) REFERENCES problems(id, organization_id) ON DELETE RESTRICT
);
CREATE INDEX problem_builds_queue ON problem_builds USING btree (created_at, id) WHERE (state = 'queued'::text);
CREATE INDEX problem_builds_recovery ON problem_builds USING btree (lease_deadline) WHERE (state = 'running'::text);

-- Snapshot inputs stay alive for the lifetime of the build.
CREATE TABLE problem_build_inputs (
    build_id uuid NOT NULL,
    organization_id uuid NOT NULL,
    object_id uuid NOT NULL,
    CONSTRAINT problem_build_inputs_pkey PRIMARY KEY (build_id, object_id),
    CONSTRAINT problem_build_inputs_build_id_fkey FOREIGN KEY (build_id, organization_id) REFERENCES problem_builds(id, organization_id) ON DELETE CASCADE,
    CONSTRAINT problem_build_inputs_object_id_fkey FOREIGN KEY (object_id, organization_id) REFERENCES stored_objects(id, organization_id) ON DELETE RESTRICT
);
CREATE INDEX problem_build_inputs_object ON problem_build_inputs USING btree (object_id);

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
    CONSTRAINT problem_packages_package_hash_check CHECK ((length((package_hash)::text) = 64)),
    CONSTRAINT problem_packages_revision_check CHECK ((revision > 0)),
    CONSTRAINT problem_packages_samples_check CHECK ((jsonb_typeof(samples) = 'array'::text)),
    CONSTRAINT problem_packages_statements_check CHECK ((jsonb_typeof(statements) = 'array'::text)),
    CONSTRAINT problem_packages_type_check CHECK ((type = ANY (ARRAY['pass-fail'::text, 'scoring'::text, 'interactive'::text]))),
    CONSTRAINT problem_packages_build_id_key UNIQUE (build_id),
    CONSTRAINT problem_packages_id_organization_id_key UNIQUE (id, organization_id),
    CONSTRAINT problem_packages_pkey PRIMARY KEY (id),
    CONSTRAINT problem_packages_build_id_fkey FOREIGN KEY (build_id) REFERENCES problem_builds(id) ON DELETE RESTRICT,
    CONSTRAINT problem_packages_object_id_fkey FOREIGN KEY (object_id, organization_id) REFERENCES stored_objects(id, organization_id) ON DELETE RESTRICT,
    CONSTRAINT problem_packages_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
    CONSTRAINT problem_packages_problem_id_fkey FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE
);
CREATE INDEX problem_packages_created_at_idx ON problem_packages USING btree (created_at DESC);
CREATE INDEX problem_packages_object ON problem_packages USING btree (object_id);
CREATE INDEX problem_packages_organization_id_idx ON problem_packages USING btree (organization_id);
CREATE INDEX problem_packages_package_hash_idx ON problem_packages USING btree (package_hash);
CREATE INDEX problem_packages_problem_id_idx ON problem_packages USING btree (problem_id);

CREATE TABLE problem_package_assets (
    package_id uuid NOT NULL,
    organization_id uuid NOT NULL,
    name text NOT NULL,
    object_id uuid NOT NULL,
    CONSTRAINT problem_package_assets_pkey PRIMARY KEY (package_id, name),
    CONSTRAINT problem_package_assets_object_id_fkey FOREIGN KEY (object_id, organization_id) REFERENCES stored_objects(id, organization_id) ON DELETE RESTRICT,
    CONSTRAINT problem_package_assets_package_id_fkey FOREIGN KEY (package_id, organization_id) REFERENCES problem_packages(id, organization_id) ON DELETE CASCADE
);
CREATE INDEX problem_package_assets_object ON problem_package_assets USING btree (object_id);

-- +goose StatementBegin
CREATE FUNCTION check_build_publication() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE bid uuid; s text; has_package boolean;
BEGIN
 IF TG_TABLE_NAME='problem_builds' THEN bid=COALESCE(NEW.id,OLD.id); ELSE bid=COALESCE(NEW.build_id,OLD.build_id); END IF;
 SELECT state INTO s FROM problem_builds WHERE id=bid;
 IF NOT FOUND THEN RETURN NULL; END IF;
 SELECT EXISTS(SELECT 1 FROM problem_packages WHERE build_id=bid) INTO has_package;
 IF (s='succeeded')<>has_package THEN RAISE EXCEPTION 'build publication must be atomic'; END IF;
 RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION guard_problem_build() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF (NEW.id,NEW.problem_id,NEW.organization_id,NEW.revision,NEW.snapshot,NEW.created_at,NEW.max_attempts,NEW.deadline)
      IS DISTINCT FROM (OLD.id,OLD.problem_id,OLD.organization_id,OLD.revision,OLD.snapshot,OLD.created_at,OLD.max_attempts,OLD.deadline) THEN
        RAISE EXCEPTION 'immutable build snapshot and settings';
    END IF;
    IF OLD.state IN ('succeeded','failed','cancelled') THEN RAISE EXCEPTION 'terminal build is immutable'; END IF;
    IF NEW.state<>OLD.state AND NOT (
        (OLD.state='queued' AND NEW.state IN ('running','failed','cancelled')) OR
        (OLD.state='running' AND NEW.state IN ('queued','succeeded','failed','cancelled'))) THEN
        RAISE EXCEPTION 'invalid build transition';
    END IF;
    IF NEW.state='running' AND OLD.state='running' AND
      (NEW.lease_token<>OLD.lease_token OR OLD.lease_deadline<=clock_timestamp()) THEN
        RAISE EXCEPTION 'cannot revive an expired lease';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION check_build_inputs() RETURNS trigger
    LANGUAGE plpgsql
    AS $_$
DECLARE bid uuid; snap jsonb; actual uuid[]; declared uuid[]; embedded uuid[];
BEGIN
    IF TG_TABLE_NAME='problem_builds' THEN bid=COALESCE(NEW.id,OLD.id); ELSE bid=COALESCE(NEW.build_id,OLD.build_id); END IF;
    SELECT snapshot INTO snap FROM problem_builds WHERE id=bid;
    IF NOT FOUND THEN RETURN NULL; END IF;
    SELECT COALESCE(array_agg(object_id ORDER BY object_id),'{}') INTO actual
      FROM problem_build_inputs WHERE build_id=bid;
    SELECT COALESCE(array_agg(DISTINCT v::uuid ORDER BY v::uuid),'{}') INTO declared
      FROM jsonb_array_elements_text(snap->'input_objects') e(v);
    SELECT COALESCE(array_agg(DISTINCT (v#>>'{}')::uuid ORDER BY (v#>>'{}')::uuid),'{}') INTO embedded FROM (
      SELECT v FROM jsonb_path_query(snap,'$.draft.tests[*].input.object_id') e(v)
      UNION ALL SELECT v FROM jsonb_path_query(snap,'$.draft.tests[*].answer.object_id') e(v)
      UNION ALL SELECT v FROM jsonb_path_query(snap,'$.draft.assets[*].object_id') e(v)
      UNION ALL SELECT v FROM jsonb_path_query(snap,'$.draft.checker.tests[*].input') e(v)
      UNION ALL SELECT v FROM jsonb_path_query(snap,'$.draft.checker.tests[*].output') e(v)
      UNION ALL SELECT v FROM jsonb_path_query(snap,'$.draft.checker.tests[*].answer') e(v)
      UNION ALL SELECT v FROM jsonb_path_query(snap,'$.draft.validators[*].tests[*].input') e(v)
    ) refs WHERE v<>'null'::jsonb;
    IF actual<>declared OR actual<>embedded THEN RAISE EXCEPTION 'snapshot input object set mismatch'; END IF;
    IF (snap->'draft'->>'revision')::bigint<>(SELECT revision FROM problem_builds WHERE id=bid)
      OR (snap->'draft'->>'problem_id')::uuid<>(SELECT problem_id FROM problem_builds WHERE id=bid)
      OR (snap->>'organization_id')::uuid<>(SELECT organization_id FROM problem_builds WHERE id=bid) THEN
        RAISE EXCEPTION 'snapshot identity mismatch';
    END IF;
    RETURN NULL;
END;
$_$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION guard_build_input() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF TG_OP='UPDATE' OR EXISTS(SELECT 1 FROM problem_builds WHERE id=OLD.build_id) THEN
        RAISE EXCEPTION 'build inputs are immutable';
    END IF;
    RETURN OLD;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION guard_package_immutable() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN RAISE EXCEPTION 'package is immutable'; END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION check_package_build() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE b problem_builds; o stored_objects;
BEGIN
    IF NEW.build_id IS NULL OR NEW.object_id IS NULL THEN RAISE EXCEPTION 'package requires build and object'; END IF;
    SELECT * INTO b FROM problem_builds WHERE id=NEW.build_id;
    SELECT * INTO o FROM stored_objects WHERE id=NEW.object_id;
    IF b.state<>'running' OR b.lease_deadline<=clock_timestamp() OR b.problem_id<>NEW.problem_id
       OR b.organization_id<>NEW.organization_id OR o.organization_id<>NEW.organization_id
       OR o.purpose<>'package' OR o.sha256<>NEW.package_hash THEN RAISE EXCEPTION 'invalid package publication'; END IF;
    NEW.revision=b.revision;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER build_publication AFTER INSERT OR UPDATE ON problem_builds DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_build_publication();
CREATE TRIGGER problem_build_guard BEFORE UPDATE ON problem_builds FOR EACH ROW EXECUTE FUNCTION guard_problem_build();
CREATE CONSTRAINT TRIGGER problem_build_inputs_snapshot AFTER INSERT OR UPDATE ON problem_builds DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_build_inputs();
CREATE TRIGGER problem_build_input_guard BEFORE DELETE OR UPDATE ON problem_build_inputs FOR EACH ROW EXECUTE FUNCTION guard_build_input();
CREATE CONSTRAINT TRIGGER problem_build_inputs_refs AFTER INSERT OR DELETE OR UPDATE ON problem_build_inputs DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_build_inputs();
CREATE TRIGGER package_immutable BEFORE UPDATE ON problem_packages FOR EACH ROW EXECUTE FUNCTION guard_package_immutable();
CREATE TRIGGER package_asset_immutable BEFORE UPDATE ON problem_package_assets FOR EACH ROW EXECUTE FUNCTION guard_package_immutable();
CREATE TRIGGER package_build BEFORE INSERT ON problem_packages FOR EACH ROW EXECUTE FUNCTION check_package_build();
CREATE CONSTRAINT TRIGGER package_publication AFTER INSERT ON problem_packages DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_build_publication();


CREATE TABLE contests (
    id uuid DEFAULT uuidv7() NOT NULL,
    organization_id uuid NOT NULL,
    visibility contest_visibility DEFAULT 'private'::contest_visibility NOT NULL,
    title text NOT NULL,
    login varchar(64) NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    participation_mode participation_mode DEFAULT 'open'::participation_mode NOT NULL,
    monitor_scope contest_scope DEFAULT 'participant'::contest_scope NOT NULL,
    submissions_list_scope contest_scope DEFAULT 'moderator'::contest_scope NOT NULL,
    submissions_review_scope contest_scope DEFAULT 'moderator'::contest_scope NOT NULL,
    submission_details_scope contest_scope DEFAULT 'moderator'::contest_scope NOT NULL,
    freeze_status freeze_status DEFAULT 'auto'::freeze_status NOT NULL,
    freeze_duration_minutes integer,
    freeze_started_at timestamptz,
    hide_statements boolean DEFAULT false NOT NULL,
    allow_clarifications boolean DEFAULT true NOT NULL,
    enable_drafts boolean DEFAULT true NOT NULL,
    enable_upsolving boolean DEFAULT true NOT NULL,
    allow_virtual boolean DEFAULT false NOT NULL,
    languages text[] DEFAULT '{}'::text[] NOT NULL,
    start_time timestamptz,
    end_time timestamptz,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT contests_check CHECK (((end_time IS NULL) OR (start_time IS NULL) OR (end_time > start_time))),
    CONSTRAINT contests_freeze_duration_check CHECK ((freeze_duration_minutes > 0)),
    CONSTRAINT contests_freeze_started_check CHECK (((freeze_status = 'frozen'::freeze_status) = (freeze_started_at IS NOT NULL))),
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
CREATE INDEX contest_members_contest_id_idx ON contest_members USING btree (contest_id);
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
CREATE INDEX contest_teams_contest_id_idx ON contest_teams USING btree (contest_id);
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
CREATE INDEX contest_problems_contest_id_idx ON contest_problems USING btree (contest_id);
CREATE INDEX contest_problems_ordinal_idx ON contest_problems USING btree (contest_id, ordinal);
CREATE INDEX contest_problems_package_id_idx ON contest_problems USING btree (package_id);
CREATE INDEX contest_problems_problem_id_idx ON contest_problems USING btree (problem_id);

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
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    contest_id uuid NOT NULL,
    user_id uuid NOT NULL,
    message text,
    status varchar(32) DEFAULT 'pending'::varchar NOT NULL,
    reviewed_by uuid,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT contest_join_requests_pkey PRIMARY KEY (id),
    CONSTRAINT contest_join_requests_contest_id_fkey FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    CONSTRAINT contest_join_requests_reviewed_by_fkey FOREIGN KEY (reviewed_by) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT contest_join_requests_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX contest_join_requests_contest_idx ON contest_join_requests USING btree (contest_id, status);
CREATE UNIQUE INDEX contest_join_requests_pending_unique_idx ON contest_join_requests USING btree (contest_id, user_id) WHERE ((status)::text = 'pending'::text);
CREATE INDEX contest_join_requests_user_idx ON contest_join_requests USING btree (user_id, status);

CREATE TABLE contest_announcements (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
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
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    contest_id uuid NOT NULL,
    problem_id uuid,
    user_id uuid NOT NULL,
    question text NOT NULL,
    answer text,
    answered_by uuid,
    status varchar(32) DEFAULT 'pending'::varchar NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    answered_at timestamptz,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT contest_clarifications_pkey PRIMARY KEY (id),
    CONSTRAINT contest_clarifications_answered_by_fkey FOREIGN KEY (answered_by) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT contest_clarifications_contest_id_fkey FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    CONSTRAINT contest_clarifications_problem_id_fkey FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE,
    CONSTRAINT contest_clarifications_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX contest_clarifications_contest_status_idx ON contest_clarifications USING btree (contest_id, status, created_at DESC);
CREATE INDEX contest_clarifications_contest_user_idx ON contest_clarifications USING btree (contest_id, user_id, created_at DESC);

CREATE TABLE contest_drafts (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
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

CREATE TABLE contest_problem_results (
    contest_id uuid NOT NULL,
    user_id uuid NOT NULL,
    problem_id uuid NOT NULL,
    solved boolean DEFAULT false NOT NULL,
    failed_attempts integer DEFAULT 0 NOT NULL,
    first_ac_time timestamptz,
    time_minutes integer,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT contest_problem_results_pkey PRIMARY KEY (contest_id, user_id, problem_id),
    CONSTRAINT contest_problem_results_contest_id_fkey FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    CONSTRAINT contest_problem_results_problem_id_fkey FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE,
    CONSTRAINT contest_problem_results_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX contest_problem_results_contest_idx ON contest_problem_results USING btree (contest_id);

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

-- +goose StatementBegin
CREATE FUNCTION check_max_problems_on_contest() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
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
-- +goose StatementEnd

CREATE TRIGGER manual_freeze_changed BEFORE INSERT OR UPDATE OF freeze_status, freeze_started_at ON contests FOR EACH ROW EXECUTE FUNCTION capture_manual_freeze();
CREATE TRIGGER on_contests_update BEFORE UPDATE ON contests FOR EACH ROW EXECUTE FUNCTION updated_at_update();
CREATE TRIGGER max_problems_on_contest_check BEFORE INSERT ON contest_problems FOR EACH ROW EXECUTE FUNCTION check_max_problems_on_contest();


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
    generation bigint DEFAULT 1 NOT NULL,
    package_id uuid,
    -- Strict mode: the time limit factor and the benchmark that measured it.
    judge_k double precision,
    judge_benchmark text,
    CONSTRAINT submissions_generation_check CHECK ((generation > 0)),
    CONSTRAINT submissions_calibration_check CHECK (((judge_k IS NULL) = (judge_benchmark IS NULL))),
    CONSTRAINT submissions_pkey PRIMARY KEY (id),
    CONSTRAINT submissions_contest_id_fkey FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE SET NULL,
    CONSTRAINT submissions_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT submissions_package_id_fkey FOREIGN KEY (package_id) REFERENCES problem_packages(id),
    CONSTRAINT submissions_problem_id_fkey FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE SET NULL
);
CREATE INDEX submissions_contest_id_idx ON submissions USING btree (contest_id);
CREATE INDEX submissions_contest_problem_idx ON submissions USING btree (contest_id, problem_id);
CREATE INDEX submissions_created_at_idx ON submissions USING btree (created_at DESC, id DESC);
CREATE INDEX submissions_owner_id_idx ON submissions USING btree (owner_id);
CREATE INDEX submissions_problem_id_idx ON submissions USING btree (problem_id);
CREATE INDEX submissions_scoreboard_idx ON submissions USING btree (contest_id, owner_id, problem_id, created_at) WHERE ((contest_id IS NOT NULL) AND (owner_id IS NOT NULL));
CREATE INDEX submissions_scoreboard_state_idx ON submissions USING btree (contest_id, owner_id, state, created_at) WHERE ((contest_id IS NOT NULL) AND (owner_id IS NOT NULL));
CREATE INDEX submissions_state_idx ON submissions USING btree (state);

CREATE TABLE judge_jobs (
    id uuid NOT NULL,
    submission_id uuid NOT NULL,
    generation bigint NOT NULL,
    package_id uuid,
    package_hash text NOT NULL,
    state text DEFAULT 'pending'::text NOT NULL,
    attempt integer DEFAULT 0 NOT NULL,
    lease_token uuid,
    deadline_at timestamptz,
    created_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT judge_jobs_attempt_check CHECK ((attempt >= 0)),
    CONSTRAINT judge_jobs_generation_check CHECK ((generation > 0)),
    CONSTRAINT judge_jobs_state_check CHECK ((state = ANY (ARRAY['pending'::text, 'processing'::text]))),
    CONSTRAINT judge_jobs_pkey PRIMARY KEY (id),
    CONSTRAINT judge_jobs_submission_id_generation_key UNIQUE (submission_id, generation),
    CONSTRAINT judge_jobs_package_id_fkey FOREIGN KEY (package_id) REFERENCES problem_packages(id) ON DELETE SET NULL,
    CONSTRAINT judge_jobs_submission_id_fkey FOREIGN KEY (submission_id) REFERENCES submissions(id) ON DELETE CASCADE
);
CREATE INDEX judge_jobs_recovery ON judge_jobs USING btree (deadline_at) WHERE (state = 'processing'::text);
CREATE INDEX judge_jobs_pending ON judge_jobs USING btree (created_at) WHERE (state = 'pending'::text);

CREATE TRIGGER on_submissions_update BEFORE UPDATE ON submissions FOR EACH ROW EXECUTE FUNCTION updated_at_update();




CREATE TABLE notifications (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
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
    author_uuid uuid NOT NULL,
    author_name text NOT NULL,
    image_key text NOT NULL,
    CONSTRAINT posts_pkey PRIMARY KEY (id)
);
CREATE INDEX idx_posts_author_uuid ON posts USING btree (author_uuid);
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
CREATE VIEW team_access AS
SELECT team_id, user_id, max(role) AS role FROM (
    SELECT team_id, user_id, role FROM team_members
    UNION ALL
    SELECT t.id, om.user_id, 'manager'::team_role
    FROM teams t JOIN organization_members om ON om.organization_id = t.organization_id
    WHERE om.role IN ('admin', 'owner')
) grants GROUP BY team_id, user_id;

CREATE VIEW problem_access AS
SELECT problem_id, user_id, max(role) AS role FROM (
    SELECT problem_id, user_id, role FROM problem_members
    UNION ALL
    SELECT link.problem_id, tm.user_id, link.role
    FROM problem_teams link JOIN team_members tm ON tm.team_id = link.team_id
    UNION ALL
    SELECT p.id, om.user_id, CASE om.role WHEN 'owner' THEN 'owner'::problem_role ELSE 'admin'::problem_role END
    FROM problems p JOIN organization_members om ON om.organization_id = p.organization_id
    WHERE om.role IN ('admin', 'owner')
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


