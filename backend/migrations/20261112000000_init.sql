-- Initial schema. Schema changes are new migrations; this file is not edited.
-- +goose Up

-- ==========================================================================
-- Extensions, types and utility functions
-- ==========================================================================

CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;

CREATE TYPE auth_token_type AS ENUM (
    'email_verification',
    'password_reset',
    'email_change'
);

CREATE TYPE contest_role AS ENUM (
    'owner',
    'moderator',
    'participant'
);

CREATE TYPE contest_visibility AS ENUM (
    'private',
    'public',
    'unlisted'
);

CREATE TYPE organization_role AS ENUM (
    'owner',
    'admin',
    'member'
);

CREATE TYPE outbox_event_status AS ENUM (
    'pending',
    'processing',
    'completed',
    'failed'
);

CREATE TYPE package_status AS ENUM (
    'pending',
    'building',
    'ready',
    'failed'
);

CREATE TYPE problem_permission AS ENUM (
    'admin',
    'write',
    'read'
);

CREATE TYPE problem_role AS ENUM (
    'owner',
    'moderator',
    'viewer'
);

CREATE TYPE problem_visibility AS ENUM (
    'private',
    'public',
    'unlisted'
);

CREATE TYPE team_role AS ENUM (
    'maintainer',
    'member'
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

-- +goose StatementBegin
CREATE FUNCTION valid_draft_path(path text) RETURNS boolean
    LANGUAGE sql IMMUTABLE
    AS $_$
    SELECT path <> '' AND path <> '.' AND path <> 'problem.yaml'
      AND path <> '.gate' AND path NOT LIKE '.gate/%'
      AND octet_length(path) <= 512 AND path !~ '(^/|/$|//|(^|/)\.{1,2}(/|$))'
      AND position(chr(92) in path) = 0;
$_$;
-- +goose StatementEnd

-- ==========================================================================
-- Users and authentication
-- ==========================================================================

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

-- ==========================================================================
-- Organizations
-- ==========================================================================

CREATE TABLE organizations (
    id uuid DEFAULT uuidv7() NOT NULL,
    login text NOT NULL,
    name text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    avatar_url text,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    join_policy varchar(32) DEFAULT 'by_request'::varchar NOT NULL,
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

-- ==========================================================================
-- Teams
-- ==========================================================================

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
    CONSTRAINT team_members_pkey PRIMARY KEY (team_id, user_id),
    CONSTRAINT team_members_team_id_fkey FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE,
    CONSTRAINT team_members_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX team_members_user_id_idx ON team_members USING btree (user_id);

CREATE TRIGGER on_teams_update BEFORE UPDATE ON teams FOR EACH ROW EXECUTE FUNCTION updated_at_update();

-- ==========================================================================
-- Stored objects and idempotent requests
-- ==========================================================================

CREATE TABLE stored_objects (
    id uuid NOT NULL,
    organization_id uuid NOT NULL,
    purpose text NOT NULL,
    key text NOT NULL,
    sha256 text NOT NULL,
    size bigint NOT NULL,
    state text NOT NULL,
    upload_deadline timestamptz NOT NULL,
    unreferenced_at timestamptz,
    created_at timestamptz DEFAULT clock_timestamp() NOT NULL,
    updated_at timestamptz DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT stored_objects_check CHECK ((key = ((('objects/'::text || (organization_id)::text) || '/'::text) || (id)::text))),
    CONSTRAINT stored_objects_purpose_check CHECK ((purpose = ANY (ARRAY['test-input'::text, 'test-answer'::text, 'asset'::text, 'package'::text]))),
    CONSTRAINT stored_objects_sha256_check CHECK ((sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT stored_objects_size_check CHECK ((size >= 0)),
    CONSTRAINT stored_objects_state_check CHECK ((state = ANY (ARRAY['uploading'::text, 'available'::text, 'deleting'::text, 'deleted'::text]))),
    CONSTRAINT stored_objects_id_organization_id_key UNIQUE (id, organization_id),
    CONSTRAINT stored_objects_key_key UNIQUE (key),
    CONSTRAINT stored_objects_pkey PRIMARY KEY (id),
    CONSTRAINT stored_objects_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE RESTRICT
);
CREATE INDEX stored_objects_gc ON stored_objects USING btree (state, unreferenced_at);

CREATE TABLE request_idempotency (
    organization_id uuid NOT NULL,
    operation text NOT NULL,
    key text NOT NULL,
    fingerprint text NOT NULL,
    token uuid NOT NULL,
    lease_deadline timestamptz NOT NULL,
    upload_id uuid,
    result jsonb,
    http_status integer,
    created_at timestamptz DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT request_idempotency_check CHECK (((result IS NULL) = (http_status IS NULL))),
    CONSTRAINT request_idempotency_fingerprint_check CHECK ((fingerprint ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT request_idempotency_http_status_check CHECK (((http_status >= 200) AND (http_status <= 299))),
    CONSTRAINT request_idempotency_key_check CHECK (((octet_length(key) >= 1) AND (octet_length(key) <= 200))),
    CONSTRAINT request_idempotency_pkey PRIMARY KEY (organization_id, operation, key),
    CONSTRAINT request_idempotency_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE RESTRICT,
    CONSTRAINT request_idempotency_upload_id_fkey FOREIGN KEY (upload_id) REFERENCES stored_objects(id) ON DELETE RESTRICT
);

CREATE TABLE request_upload_objects (
    organization_id uuid NOT NULL,
    operation text NOT NULL,
    key text NOT NULL,
    ordinal integer NOT NULL,
    object_id uuid NOT NULL,
    CONSTRAINT request_upload_objects_ordinal_check CHECK ((ordinal > 0)),
    CONSTRAINT request_upload_objects_object_id_key UNIQUE (object_id),
    CONSTRAINT request_upload_objects_pkey PRIMARY KEY (organization_id, operation, key, ordinal),
    CONSTRAINT request_upload_objects_object_id_fkey FOREIGN KEY (object_id) REFERENCES stored_objects(id) ON DELETE RESTRICT,
    CONSTRAINT request_upload_objects_organization_id_operation_key_fkey FOREIGN KEY (organization_id, operation, key) REFERENCES request_idempotency(organization_id, operation, key) ON DELETE RESTRICT
);

-- +goose StatementBegin
CREATE FUNCTION guard_stored_object() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF TG_OP='INSERT' THEN
        IF NEW.state <> 'uploading' THEN RAISE EXCEPTION 'objects start uploading'; END IF;
        RETURN NEW;
    END IF;
    IF (NEW.id,NEW.organization_id,NEW.purpose,NEW.key,NEW.sha256,NEW.size,NEW.upload_deadline,NEW.created_at)
       IS DISTINCT FROM (OLD.id,OLD.organization_id,OLD.purpose,OLD.key,OLD.sha256,OLD.size,OLD.upload_deadline,OLD.created_at) THEN
       RAISE EXCEPTION 'immutable stored object identity';
    END IF;
    IF NEW.state<>OLD.state AND NOT (
        (OLD.state='uploading' AND NEW.state IN ('available','deleting')) OR
        (OLD.state='available' AND NEW.state='deleting') OR
        (OLD.state='deleting' AND NEW.state='deleted')) THEN
        RAISE EXCEPTION 'invalid object transition';
    END IF;
    IF NEW.state='available' AND OLD.state='uploading' AND NEW.upload_deadline<=clock_timestamp() THEN
        RAISE EXCEPTION 'upload expired';
    END IF;
    NEW.updated_at=clock_timestamp();
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER stored_object_guard BEFORE INSERT OR UPDATE ON stored_objects FOR EACH ROW EXECUTE FUNCTION guard_stored_object();

-- ==========================================================================
-- Problems
-- ==========================================================================

CREATE TABLE problems (
    id uuid DEFAULT uuidv7() NOT NULL,
    organization_id uuid NOT NULL,
    owner_id uuid,
    visibility problem_visibility DEFAULT 'private'::problem_visibility NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    is_template boolean DEFAULT false NOT NULL,
    CONSTRAINT problems_id_organization_unique UNIQUE (id, organization_id),
    CONSTRAINT problems_pkey PRIMARY KEY (id),
    CONSTRAINT problems_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
    CONSTRAINT problems_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX problems_created_at_idx ON problems USING btree (created_at DESC);
CREATE INDEX problems_organization_id_idx ON problems USING btree (organization_id);
CREATE INDEX problems_owner_id_idx ON problems USING btree (owner_id);
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

CREATE TABLE problem_teams (
    problem_id uuid NOT NULL,
    team_id uuid NOT NULL,
    permission problem_permission DEFAULT 'read'::problem_permission NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT problem_teams_pkey PRIMARY KEY (problem_id, team_id),
    CONSTRAINT problem_teams_problem_id_fkey FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE,
    CONSTRAINT problem_teams_team_id_fkey FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE
);
CREATE INDEX problem_teams_problem_id_idx ON problem_teams USING btree (problem_id);
CREATE INDEX problem_teams_team_id_idx ON problem_teams USING btree (team_id);

CREATE TRIGGER on_problems_update BEFORE UPDATE ON problems FOR EACH ROW EXECUTE FUNCTION updated_at_update();

-- ==========================================================================
-- Problem drafts
-- ==========================================================================

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
    checker_id uuid,
    validator_id uuid,
    interactor_id uuid,
    main_solution_id uuid,
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
    CONSTRAINT problem_drafts_problem_id_organization_id_fkey FOREIGN KEY (problem_id, organization_id) REFERENCES problems(id, organization_id) ON DELETE CASCADE
);
CREATE INDEX problem_drafts_title_trgm ON problem_drafts USING gin (title gin_trgm_ops);

CREATE TABLE problem_draft_files (
    problem_id uuid NOT NULL,
    path text NOT NULL,
    content text NOT NULL,
    CONSTRAINT problem_draft_files_path_check CHECK (valid_draft_path(path)),
    CONSTRAINT problem_draft_files_pkey PRIMARY KEY (problem_id, path),
    CONSTRAINT problem_draft_files_problem_id_fkey FOREIGN KEY (problem_id) REFERENCES problem_drafts(problem_id) ON DELETE CASCADE
);

CREATE TABLE problem_draft_components (
    problem_id uuid NOT NULL,
    id uuid NOT NULL,
    role text NOT NULL,
    path text NOT NULL,
    language text NOT NULL,
    dependencies jsonb NOT NULL,
    CONSTRAINT problem_draft_components_dependencies_check CHECK ((jsonb_typeof(dependencies) = 'object'::text)),
    CONSTRAINT problem_draft_components_language_check CHECK ((language <> ''::text)),
    CONSTRAINT problem_draft_components_role_check CHECK ((role = ANY (ARRAY['checker'::text, 'validator'::text, 'interactor'::text, 'generator'::text, 'solution'::text]))),
    CONSTRAINT problem_draft_components_pkey PRIMARY KEY (problem_id, id),
    CONSTRAINT problem_draft_components_problem_id_id_role_key UNIQUE (problem_id, id, role),
    CONSTRAINT problem_draft_components_problem_id_fkey FOREIGN KEY (problem_id) REFERENCES problem_drafts(problem_id) ON DELETE CASCADE,
    CONSTRAINT problem_draft_components_problem_id_path_fkey FOREIGN KEY (problem_id, path) REFERENCES problem_draft_files(problem_id, path) DEFERRABLE INITIALLY DEFERRED
);

CREATE TABLE problem_draft_assets (
    problem_id uuid NOT NULL,
    path text NOT NULL,
    object_id uuid NOT NULL,
    CONSTRAINT problem_draft_assets_path_check CHECK (valid_draft_path(path)),
    CONSTRAINT problem_draft_assets_pkey PRIMARY KEY (problem_id, path),
    CONSTRAINT problem_draft_assets_object_id_fkey FOREIGN KEY (object_id) REFERENCES stored_objects(id) ON DELETE RESTRICT,
    CONSTRAINT problem_draft_assets_problem_id_fkey FOREIGN KEY (problem_id) REFERENCES problem_drafts(problem_id) ON DELETE CASCADE
);
CREATE INDEX problem_draft_assets_object ON problem_draft_assets USING btree (object_id);

CREATE TABLE problem_draft_subtasks (
    problem_id uuid NOT NULL,
    id uuid NOT NULL,
    ordinal integer NOT NULL,
    name text NOT NULL,
    points numeric(12,2) NOT NULL,
    policy text NOT NULL,
    CONSTRAINT problem_draft_subtasks_ordinal_check CHECK ((ordinal > 0)),
    CONSTRAINT problem_draft_subtasks_points_check CHECK ((points >= (0)::numeric)),
    CONSTRAINT problem_draft_subtasks_policy_check CHECK ((policy = ANY (ARRAY['complete'::text, 'each'::text]))),
    CONSTRAINT problem_draft_subtasks_pkey PRIMARY KEY (problem_id, id),
    CONSTRAINT problem_draft_subtasks_problem_id_ordinal_key UNIQUE (problem_id, ordinal) DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT problem_draft_subtasks_problem_id_fkey FOREIGN KEY (problem_id) REFERENCES problem_drafts(problem_id) ON DELETE CASCADE
);

CREATE TABLE problem_draft_subtask_dependencies (
    problem_id uuid NOT NULL,
    subtask_id uuid NOT NULL,
    depends_on uuid NOT NULL,
    CONSTRAINT problem_draft_subtask_dependencies_check CHECK ((subtask_id <> depends_on)),
    CONSTRAINT problem_draft_subtask_dependencies_pkey PRIMARY KEY (problem_id, subtask_id, depends_on),
    CONSTRAINT problem_draft_subtask_dependencies_problem_id_depends_on_fkey FOREIGN KEY (problem_id, depends_on) REFERENCES problem_draft_subtasks(problem_id, id) DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT problem_draft_subtask_dependencies_problem_id_subtask_id_fkey FOREIGN KEY (problem_id, subtask_id) REFERENCES problem_draft_subtasks(problem_id, id) ON DELETE CASCADE
);

CREATE TABLE problem_draft_tests (
    problem_id uuid NOT NULL,
    id uuid NOT NULL,
    ordinal integer NOT NULL,
    subtask_id uuid NOT NULL,
    sample boolean NOT NULL,
    points numeric(12,2) NOT NULL,
    input_mode text NOT NULL,
    input_object_id uuid,
    generator_id uuid,
    generator_args text[] NOT NULL,
    seed bigint,
    answer_mode text NOT NULL,
    answer_object_id uuid,
    solution_id uuid,
    CONSTRAINT problem_draft_tests_answer_mode_check CHECK ((answer_mode = ANY (ARRAY['provided'::text, 'generated'::text, 'interactive'::text]))),
    CONSTRAINT problem_draft_tests_check CHECK ((((input_mode = 'manual'::text) AND (input_object_id IS NOT NULL) AND (generator_id IS NULL) AND (seed IS NULL) AND (cardinality(generator_args) = 0)) OR ((input_mode = 'generator'::text) AND (input_object_id IS NULL) AND (generator_id IS NOT NULL) AND (seed IS NOT NULL)))),
    CONSTRAINT problem_draft_tests_check1 CHECK ((((answer_mode = 'provided'::text) AND (answer_object_id IS NOT NULL) AND (solution_id IS NULL)) OR ((answer_mode = 'generated'::text) AND (answer_object_id IS NULL) AND (solution_id IS NOT NULL)) OR ((answer_mode = 'interactive'::text) AND (answer_object_id IS NULL) AND (solution_id IS NULL)))),
    CONSTRAINT problem_draft_tests_input_mode_check CHECK ((input_mode = ANY (ARRAY['manual'::text, 'generator'::text]))),
    CONSTRAINT problem_draft_tests_ordinal_check CHECK ((ordinal > 0)),
    CONSTRAINT problem_draft_tests_points_check CHECK ((points >= (0)::numeric)),
    CONSTRAINT problem_draft_tests_pkey PRIMARY KEY (problem_id, id),
    CONSTRAINT problem_draft_tests_problem_id_ordinal_key UNIQUE (problem_id, ordinal) DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT problem_draft_tests_answer_object_id_fkey FOREIGN KEY (answer_object_id) REFERENCES stored_objects(id) ON DELETE RESTRICT,
    CONSTRAINT problem_draft_tests_input_object_id_fkey FOREIGN KEY (input_object_id) REFERENCES stored_objects(id) ON DELETE RESTRICT,
    CONSTRAINT problem_draft_tests_problem_id_fkey FOREIGN KEY (problem_id) REFERENCES problem_drafts(problem_id) ON DELETE CASCADE,
    CONSTRAINT problem_draft_tests_problem_id_generator_id_fkey FOREIGN KEY (problem_id, generator_id) REFERENCES problem_draft_components(problem_id, id) DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT problem_draft_tests_problem_id_solution_id_fkey FOREIGN KEY (problem_id, solution_id) REFERENCES problem_draft_components(problem_id, id) DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT problem_draft_tests_problem_id_subtask_id_fkey FOREIGN KEY (problem_id, subtask_id) REFERENCES problem_draft_subtasks(problem_id, id) DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX problem_draft_tests_answer ON problem_draft_tests USING btree (answer_object_id);
CREATE INDEX problem_draft_tests_input ON problem_draft_tests USING btree (input_object_id);

-- Cyclic references, added once both tables exist.
ALTER TABLE problem_drafts ADD CONSTRAINT problem_drafts_problem_id_checker_id_fkey FOREIGN KEY (problem_id, checker_id) REFERENCES problem_draft_components(problem_id, id) DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE problem_drafts ADD CONSTRAINT problem_drafts_problem_id_interactor_id_fkey FOREIGN KEY (problem_id, interactor_id) REFERENCES problem_draft_components(problem_id, id) DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE problem_drafts ADD CONSTRAINT problem_drafts_problem_id_main_solution_id_fkey FOREIGN KEY (problem_id, main_solution_id) REFERENCES problem_draft_components(problem_id, id) DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE problem_drafts ADD CONSTRAINT problem_drafts_problem_id_validator_id_fkey FOREIGN KEY (problem_id, validator_id) REFERENCES problem_draft_components(problem_id, id) DEFERRABLE INITIALLY DEFERRED;

CREATE VIEW problem_catalog AS
 SELECT p.id,
    p.organization_id,
    p.owner_id,
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

-- ==========================================================================
-- Problem builds and packages
-- ==========================================================================

CREATE TABLE problem_builds (
    id uuid NOT NULL,
    problem_id uuid NOT NULL,
    organization_id uuid NOT NULL,
    revision bigint NOT NULL,
    snapshot jsonb NOT NULL,
    schema_version integer NOT NULL,
    toolchain_digest text NOT NULL,
    state text NOT NULL,
    lease_token uuid,
    lease_deadline timestamptz,
    attempt integer DEFAULT 0 NOT NULL,
    max_attempts integer NOT NULL,
    deadline timestamptz NOT NULL,
    diagnostic jsonb,
    created_at timestamptz DEFAULT clock_timestamp() NOT NULL,
    finished_at timestamptz,
    CONSTRAINT problem_builds_attempt_check CHECK ((attempt >= 0)),
    CONSTRAINT problem_builds_check CHECK (((state = 'running'::text) = ((lease_token IS NOT NULL) AND (lease_deadline IS NOT NULL)))),
    CONSTRAINT problem_builds_check1 CHECK (((state = ANY (ARRAY['succeeded'::text, 'failed'::text, 'cancelled'::text])) = (finished_at IS NOT NULL))),
    CONSTRAINT problem_builds_diagnostic_check CHECK ((octet_length((diagnostic)::text) <= 131072)),
    CONSTRAINT problem_builds_max_attempts_check CHECK ((max_attempts > 0)),
    CONSTRAINT problem_builds_revision_check CHECK ((revision > 0)),
    CONSTRAINT problem_builds_schema_version_check CHECK ((schema_version = 1)),
    CONSTRAINT problem_builds_snapshot_check CHECK ((jsonb_typeof(snapshot) = 'object'::text)),
    CONSTRAINT problem_builds_state_check CHECK ((state = ANY (ARRAY['queued'::text, 'running'::text, 'succeeded'::text, 'failed'::text, 'cancelled'::text]))),
    CONSTRAINT problem_builds_toolchain_digest_check CHECK ((toolchain_digest ~ '^sha256:[0-9a-f]{64}$'::text)),
    CONSTRAINT problem_builds_id_problem_id_organization_id_key UNIQUE (id, problem_id, organization_id),
    CONSTRAINT problem_builds_pkey PRIMARY KEY (id),
    CONSTRAINT problem_builds_problem_id_organization_id_fkey FOREIGN KEY (problem_id, organization_id) REFERENCES problems(id, organization_id) ON DELETE RESTRICT
);
CREATE INDEX problem_builds_queue ON problem_builds USING btree (created_at, id) WHERE (state = 'queued'::text);
CREATE INDEX problem_builds_recovery ON problem_builds USING btree (lease_deadline) WHERE (state = 'running'::text);

CREATE TABLE problem_build_objects (
    build_id uuid NOT NULL,
    object_id uuid NOT NULL,
    role text NOT NULL,
    attempt_token uuid,
    CONSTRAINT problem_build_objects_check CHECK ((((role = 'input'::text) AND (attempt_token IS NULL)) OR ((role = 'output'::text) AND (attempt_token IS NOT NULL)))),
    CONSTRAINT problem_build_objects_role_check CHECK ((role = ANY (ARRAY['input'::text, 'output'::text]))),
    CONSTRAINT problem_build_objects_pkey PRIMARY KEY (build_id, object_id, role),
    CONSTRAINT problem_build_objects_build_id_fkey FOREIGN KEY (build_id) REFERENCES problem_builds(id) ON DELETE CASCADE,
    CONSTRAINT problem_build_objects_object_id_fkey FOREIGN KEY (object_id) REFERENCES stored_objects(id) ON DELETE RESTRICT
);
CREATE INDEX problem_build_objects_object ON problem_build_objects USING btree (object_id);

CREATE TABLE problem_packages (
    id uuid DEFAULT uuidv7() NOT NULL,
    problem_id uuid NOT NULL,
    organization_id uuid NOT NULL,
    package_hash varchar(64) NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    title text DEFAULT ''::text NOT NULL,
    short_name text DEFAULT ''::text NOT NULL,
    time_limit_ms integer DEFAULT 1000 NOT NULL,
    memory_limit_mb integer DEFAULT 256 NOT NULL,
    build_id uuid NOT NULL,
    object_id uuid NOT NULL,
    revision bigint NOT NULL,
    CONSTRAINT problem_packages_package_hash_check CHECK ((length((package_hash)::text) = 64)),
    CONSTRAINT problem_packages_revision_check CHECK ((revision > 0)),
    CONSTRAINT problem_packages_build_id_key UNIQUE (build_id),
    CONSTRAINT problem_packages_pkey PRIMARY KEY (id),
    CONSTRAINT problem_packages_build_id_fkey FOREIGN KEY (build_id) REFERENCES problem_builds(id) ON DELETE RESTRICT,
    CONSTRAINT problem_packages_object_id_fkey FOREIGN KEY (object_id) REFERENCES stored_objects(id) ON DELETE RESTRICT,
    CONSTRAINT problem_packages_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
    CONSTRAINT problem_packages_problem_id_fkey FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE
);
CREATE INDEX problem_packages_created_at_idx ON problem_packages USING btree (created_at DESC);
CREATE INDEX problem_packages_object ON problem_packages USING btree (object_id);
CREATE INDEX problem_packages_organization_id_idx ON problem_packages USING btree (organization_id);
CREATE INDEX problem_packages_package_hash_idx ON problem_packages USING btree (package_hash);
CREATE INDEX problem_packages_problem_id_idx ON problem_packages USING btree (problem_id);

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
    IF (NEW.id,NEW.problem_id,NEW.organization_id,NEW.revision,NEW.snapshot,NEW.schema_version,NEW.toolchain_digest,NEW.created_at,NEW.max_attempts,NEW.deadline)
      IS DISTINCT FROM (OLD.id,OLD.problem_id,OLD.organization_id,OLD.revision,OLD.snapshot,OLD.schema_version,OLD.toolchain_digest,OLD.created_at,OLD.max_attempts,OLD.deadline) THEN
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
      FROM problem_build_objects WHERE build_id=bid AND role='input';
    SELECT COALESCE(array_agg(DISTINCT v::uuid ORDER BY v::uuid),'{}') INTO declared
      FROM jsonb_array_elements_text(snap->'input_objects') e(v);
    SELECT COALESCE(array_agg(DISTINCT (v#>>'{}')::uuid ORDER BY (v#>>'{}')::uuid),'{}') INTO embedded FROM (
      SELECT v FROM jsonb_path_query(snap,'$.draft.tests[*].input.object_id') e(v)
      UNION ALL SELECT v FROM jsonb_path_query(snap,'$.draft.tests[*].answer.object_id') e(v)
      UNION ALL SELECT v FROM jsonb_path_query(snap,'$.draft.assets[*].object_id') e(v)
    ) refs WHERE v<>'null'::jsonb;
    IF actual<>declared OR actual<>embedded THEN RAISE EXCEPTION 'snapshot input object set mismatch'; END IF;
    IF (snap->>'schema_version')::integer<>1 OR (snap->'draft'->>'revision')::bigint<>(SELECT revision FROM problem_builds WHERE id=bid)
      OR (snap->'draft'->>'problem_id')::uuid<>(SELECT problem_id FROM problem_builds WHERE id=bid)
      OR (snap->>'organization_id')::uuid<>(SELECT organization_id FROM problem_builds WHERE id=bid)
      OR snap->'toolchain'->>'digest'<>(SELECT toolchain_digest FROM problem_builds WHERE id=bid) THEN
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
    IF OLD.role='input' AND EXISTS(SELECT 1 FROM problem_builds WHERE id=OLD.build_id) THEN
        RAISE EXCEPTION 'build inputs are immutable';
    END IF;
    IF TG_OP='UPDATE' THEN RAISE EXCEPTION 'build object references cannot be updated'; END IF;
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
CREATE FUNCTION capture_package_metadata() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE b problem_builds; o stored_objects;
BEGIN
    IF NEW.build_id IS NULL OR NEW.object_id IS NULL THEN RAISE EXCEPTION 'package requires build and object'; END IF;
    SELECT * INTO b FROM problem_builds WHERE id=NEW.build_id;
    SELECT * INTO o FROM stored_objects WHERE id=NEW.object_id;
    IF b.state<>'running' OR b.lease_deadline<=clock_timestamp() OR b.problem_id<>NEW.problem_id
       OR b.organization_id<>NEW.organization_id OR o.organization_id<>NEW.organization_id OR o.state<>'available'
       OR o.purpose<>'package' OR o.sha256<>NEW.package_hash THEN RAISE EXCEPTION 'invalid package publication'; END IF;
    NEW.revision=b.revision;
    NEW.title=b.snapshot->'draft'->>'title';
    NEW.short_name=b.snapshot->'draft'->>'short_name';
    NEW.time_limit_ms=(b.snapshot->'draft'->>'time_limit_ms')::integer;
    NEW.memory_limit_mb=(b.snapshot->'draft'->>'memory_limit_mb')::integer;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER build_publication AFTER INSERT OR UPDATE ON problem_builds DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_build_publication();
CREATE TRIGGER problem_build_guard BEFORE UPDATE ON problem_builds FOR EACH ROW EXECUTE FUNCTION guard_problem_build();
CREATE CONSTRAINT TRIGGER problem_build_inputs_snapshot AFTER INSERT OR UPDATE ON problem_builds DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_build_inputs();
CREATE TRIGGER problem_build_input_guard BEFORE DELETE OR UPDATE ON problem_build_objects FOR EACH ROW EXECUTE FUNCTION guard_build_input();
CREATE CONSTRAINT TRIGGER problem_build_inputs_refs AFTER INSERT OR DELETE OR UPDATE ON problem_build_objects DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_build_inputs();
CREATE TRIGGER package_immutable BEFORE UPDATE ON problem_packages FOR EACH ROW EXECUTE FUNCTION guard_package_immutable();
CREATE TRIGGER package_metadata BEFORE INSERT ON problem_packages FOR EACH ROW EXECUTE FUNCTION capture_package_metadata();
CREATE CONSTRAINT TRIGGER package_publication AFTER INSERT ON problem_packages DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_build_publication();

-- ==========================================================================
-- Contests
-- ==========================================================================

CREATE TABLE contests (
    id uuid DEFAULT uuidv7() NOT NULL,
    organization_id uuid NOT NULL,
    owner_id uuid,
    visibility contest_visibility DEFAULT 'private'::contest_visibility NOT NULL,
    title text NOT NULL,
    login varchar(64) NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    settings jsonb NOT NULL,
    start_time timestamptz,
    end_time timestamptz,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT contests_check CHECK (((end_time IS NULL) OR (start_time IS NULL) OR (end_time > start_time))),
    CONSTRAINT contests_login_check CHECK (((length((login)::text) >= 3) AND (length((login)::text) <= 64))),
    CONSTRAINT contests_pkey PRIMARY KEY (id),
    CONSTRAINT contests_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
    CONSTRAINT contests_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX contests_created_at_idx ON contests USING btree (created_at DESC);
CREATE UNIQUE INDEX contests_org_id_login_lower_idx ON contests USING btree (organization_id, lower((login)::text));
CREATE INDEX contests_organization_id_idx ON contests USING btree (organization_id);
CREATE INDEX contests_owner_id_idx ON contests USING btree (owner_id);
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
    role contest_role DEFAULT 'participant'::contest_role NOT NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT contest_teams_pkey PRIMARY KEY (contest_id, team_id),
    CONSTRAINT contest_teams_contest_id_fkey FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    CONSTRAINT contest_teams_team_id_fkey FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE
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

-- +goose StatementBegin
CREATE FUNCTION capture_manual_freeze() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  IF NEW.settings->>'freeze_status'='frozen' THEN
    IF TG_OP='INSERT' OR OLD.settings->>'freeze_status' IS DISTINCT FROM 'frozen' THEN
      NEW.settings=jsonb_set(NEW.settings,'{freeze_started_at}',to_jsonb(clock_timestamp()));
    ELSE
      NEW.settings=jsonb_set(NEW.settings,'{freeze_started_at}',OLD.settings->'freeze_started_at');
    END IF;
  ELSE
    NEW.settings=NEW.settings-'freeze_started_at';
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

CREATE TRIGGER manual_freeze_changed BEFORE INSERT OR UPDATE OF settings ON contests FOR EACH ROW EXECUTE FUNCTION capture_manual_freeze();
CREATE TRIGGER on_contests_update BEFORE UPDATE ON contests FOR EACH ROW EXECUTE FUNCTION updated_at_update();
CREATE TRIGGER max_problems_on_contest_check BEFORE INSERT ON contest_problems FOR EACH ROW EXECUTE FUNCTION check_max_problems_on_contest();

-- ==========================================================================
-- Submissions and judging
-- ==========================================================================

CREATE TABLE submissions (
    id uuid DEFAULT uuidv7() NOT NULL,
    contest_id uuid,
    problem_id uuid,
    owner_id uuid,
    source varchar(65536) NOT NULL,
    language integer NOT NULL,
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
    CONSTRAINT submissions_generation_check CHECK ((generation > 0)),
    CONSTRAINT submissions_pkey PRIMARY KEY (id),
    CONSTRAINT submissions_contest_id_fkey FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE SET NULL,
    CONSTRAINT submissions_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT submissions_package_id_fkey FOREIGN KEY (package_id) REFERENCES problem_packages(id),
    CONSTRAINT submissions_problem_id_fkey FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE SET NULL
);
CREATE INDEX submissions_contest_id_idx ON submissions USING btree (contest_id);
CREATE INDEX submissions_contest_problem_idx ON submissions USING btree (contest_id, problem_id);
CREATE INDEX submissions_created_at_idx ON submissions USING btree (created_at DESC);
CREATE INDEX submissions_owner_id_idx ON submissions USING btree (owner_id);
CREATE INDEX submissions_problem_id_idx ON submissions USING btree (problem_id);
CREATE INDEX submissions_scoreboard_idx ON submissions USING btree (contest_id, owner_id, problem_id, created_at) WHERE ((contest_id IS NOT NULL) AND (owner_id IS NOT NULL));
CREATE INDEX submissions_scoreboard_state_idx ON submissions USING btree (contest_id, owner_id, state, created_at) WHERE ((contest_id IS NOT NULL) AND (owner_id IS NOT NULL));
CREATE INDEX submissions_state_idx ON submissions USING btree (state);

CREATE TABLE submission_state_history (
    id bigint GENERATED ALWAYS AS IDENTITY,
    submission_id uuid NOT NULL,
    observed_at timestamptz NOT NULL,
    state integer NOT NULL,
    CONSTRAINT submission_state_history_pkey PRIMARY KEY (id),
    CONSTRAINT submission_state_history_submission_id_fkey FOREIGN KEY (submission_id) REFERENCES submissions(id) ON DELETE CASCADE
);
CREATE INDEX submission_state_history_lookup ON submission_state_history USING btree (submission_id, observed_at DESC, id DESC);

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
    completed_at timestamptz,
    CONSTRAINT judge_jobs_attempt_check CHECK ((attempt >= 0)),
    CONSTRAINT judge_jobs_generation_check CHECK ((generation > 0)),
    CONSTRAINT judge_jobs_state_check CHECK ((state = ANY (ARRAY['pending'::text, 'processing'::text, 'completed'::text]))),
    CONSTRAINT judge_jobs_pkey PRIMARY KEY (id),
    CONSTRAINT judge_jobs_submission_id_generation_key UNIQUE (submission_id, generation),
    CONSTRAINT judge_jobs_package_id_fkey FOREIGN KEY (package_id) REFERENCES problem_packages(id) ON DELETE SET NULL,
    CONSTRAINT judge_jobs_submission_id_fkey FOREIGN KEY (submission_id) REFERENCES submissions(id) ON DELETE CASCADE
);
CREATE INDEX judge_jobs_recovery ON judge_jobs USING btree (deadline_at) WHERE (state = 'processing'::text);

-- +goose StatementBegin
CREATE FUNCTION capture_submission_state() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  INSERT INTO submission_state_history(submission_id,observed_at,state) VALUES(NEW.id,clock_timestamp(),NEW.state);
  RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER on_submissions_update BEFORE UPDATE ON submissions FOR EACH ROW EXECUTE FUNCTION updated_at_update();
CREATE TRIGGER submission_state_changed AFTER INSERT OR UPDATE OF state ON submissions FOR EACH ROW EXECUTE FUNCTION capture_submission_state();

-- +goose StatementBegin
CREATE FUNCTION get_contest_scoreboard(p_contest_id uuid, p_realtime boolean DEFAULT false) RETURNS TABLE(user_id uuid, username varchar, problems_solved integer, total_penalty integer, last_accepted_at timestamptz, problem_results jsonb)
    LANGUAGE plpgsql STABLE
    AS $$
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
-- +goose StatementEnd

-- ==========================================================================
-- Messaging: outbox, inbox and event aggregates
-- ==========================================================================

CREATE TABLE outbox_events (
    id uuid DEFAULT uuidv7() NOT NULL,
    aggregate_id uuid NOT NULL,
    event_type varchar(64) NOT NULL,
    payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    status outbox_event_status DEFAULT 'pending'::outbox_event_status NOT NULL,
    retry_count integer DEFAULT 0 NOT NULL,
    error_message text,
    created_at timestamptz DEFAULT now() NOT NULL,
    processed_at timestamptz,
    locked_at timestamptz,
    deadline_at timestamptz,
    headers jsonb DEFAULT '{}'::jsonb NOT NULL,
    aggregate_version bigint NOT NULL,
    lease_token uuid,
    next_attempt_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT outbox_events_retry_count_check CHECK ((retry_count >= 0)),
    CONSTRAINT outbox_events_pkey PRIMARY KEY (id)
);
CREATE UNIQUE INDEX outbox_aggregate_version ON outbox_events USING btree (aggregate_id, aggregate_version);
CREATE INDEX outbox_events_aggregate_id_idx ON outbox_events USING btree (aggregate_id);
CREATE INDEX outbox_events_status_idx ON outbox_events USING btree (status);
CREATE INDEX outbox_events_worker_idx ON outbox_events USING btree (created_at) WHERE ((status = 'pending'::outbox_event_status) OR (status = 'processing'::outbox_event_status));

CREATE TABLE inbox_receipts (
    consumer_name text NOT NULL,
    event_id uuid NOT NULL,
    processed_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT inbox_receipts_pkey PRIMARY KEY (consumer_name, event_id)
);

CREATE TABLE event_aggregates (
    aggregate_id uuid NOT NULL,
    version bigint NOT NULL,
    CONSTRAINT event_aggregates_version_check CHECK ((version > 0)),
    CONSTRAINT event_aggregates_pkey PRIMARY KEY (aggregate_id)
);

-- ==========================================================================
-- Notifications and posts
-- ==========================================================================

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

-- ==========================================================================
-- Access revision: any change to access-relevant tables bumps one counter
-- ==========================================================================

CREATE TABLE access_revision (
    singleton boolean DEFAULT true NOT NULL,
    revision bigint DEFAULT 1 NOT NULL,
    CONSTRAINT access_revision_singleton_check CHECK (singleton),
    CONSTRAINT access_revision_pkey PRIMARY KEY (singleton)
);

INSERT INTO access_revision DEFAULT VALUES;

-- +goose StatementBegin
CREATE FUNCTION bump_access_revision() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    UPDATE access_revision SET revision=revision+1;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER access_revision_changed AFTER INSERT OR DELETE OR UPDATE ON users FOR EACH STATEMENT EXECUTE FUNCTION bump_access_revision();
CREATE TRIGGER access_revision_changed AFTER INSERT OR DELETE OR UPDATE ON organizations FOR EACH STATEMENT EXECUTE FUNCTION bump_access_revision();
CREATE TRIGGER access_revision_changed AFTER INSERT OR DELETE OR UPDATE ON organization_members FOR EACH STATEMENT EXECUTE FUNCTION bump_access_revision();
CREATE TRIGGER access_revision_changed AFTER INSERT OR DELETE OR UPDATE ON teams FOR EACH STATEMENT EXECUTE FUNCTION bump_access_revision();
CREATE TRIGGER access_revision_changed AFTER INSERT OR DELETE OR UPDATE ON team_members FOR EACH STATEMENT EXECUTE FUNCTION bump_access_revision();
CREATE TRIGGER access_revision_changed AFTER INSERT OR DELETE OR UPDATE ON problems FOR EACH STATEMENT EXECUTE FUNCTION bump_access_revision();
CREATE TRIGGER access_revision_changed AFTER INSERT OR DELETE OR UPDATE ON problem_members FOR EACH STATEMENT EXECUTE FUNCTION bump_access_revision();
CREATE TRIGGER access_revision_changed AFTER INSERT OR DELETE OR UPDATE ON problem_teams FOR EACH STATEMENT EXECUTE FUNCTION bump_access_revision();
CREATE TRIGGER access_revision_changed AFTER INSERT OR DELETE OR UPDATE ON problem_packages FOR EACH STATEMENT EXECUTE FUNCTION bump_access_revision();
CREATE TRIGGER access_revision_changed AFTER INSERT OR DELETE OR UPDATE ON contests FOR EACH STATEMENT EXECUTE FUNCTION bump_access_revision();
CREATE TRIGGER access_revision_changed AFTER INSERT OR DELETE OR UPDATE ON contest_members FOR EACH STATEMENT EXECUTE FUNCTION bump_access_revision();
CREATE TRIGGER access_revision_changed AFTER INSERT OR DELETE OR UPDATE ON contest_teams FOR EACH STATEMENT EXECUTE FUNCTION bump_access_revision();
CREATE TRIGGER access_revision_changed AFTER INSERT OR DELETE OR UPDATE ON contest_problems FOR EACH STATEMENT EXECUTE FUNCTION bump_access_revision();
CREATE TRIGGER access_revision_changed AFTER INSERT OR DELETE OR UPDATE ON contest_user_problem_blocks FOR EACH STATEMENT EXECUTE FUNCTION bump_access_revision();

-- ==========================================================================
-- Stored object references: objects become unreferenced when no row points to them
-- ==========================================================================

-- +goose StatementBegin
CREATE FUNCTION stored_object_has_references(oid uuid) RETURNS boolean
    LANGUAGE sql
    AS $$
 SELECT EXISTS(SELECT 1 FROM problem_draft_tests WHERE input_object_id=oid OR answer_object_id=oid)
 OR EXISTS(SELECT 1 FROM problem_draft_assets WHERE object_id=oid)
 OR EXISTS(SELECT 1 FROM problem_build_objects WHERE object_id=oid)
 OR EXISTS(SELECT 1 FROM problem_packages WHERE object_id=oid)
 OR EXISTS(SELECT 1 FROM request_idempotency WHERE upload_id=oid)
 OR EXISTS(SELECT 1 FROM request_upload_objects WHERE object_id=oid);
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION maintain_stored_object_references() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE old_ids uuid[]='{}'; new_ids uuid[]='{}'; ids uuid[]; oid uuid; owner_org uuid; obj stored_objects; is_output boolean=false;
BEGIN
 IF TG_TABLE_NAME='problem_draft_tests' THEN
  IF TG_OP<>'INSERT' THEN old_ids=ARRAY[OLD.input_object_id,OLD.answer_object_id]; END IF;
  IF TG_OP<>'DELETE' THEN
   new_ids=ARRAY[NEW.input_object_id,NEW.answer_object_id];
   SELECT organization_id INTO owner_org FROM problems WHERE id=NEW.problem_id;
  END IF;
 ELSIF TG_TABLE_NAME='problem_draft_assets' THEN
  IF TG_OP<>'INSERT' THEN old_ids=ARRAY[OLD.object_id]; END IF;
  IF TG_OP<>'DELETE' THEN
   new_ids=ARRAY[NEW.object_id]; SELECT organization_id INTO owner_org FROM problems WHERE id=NEW.problem_id;
  END IF;
 ELSIF TG_TABLE_NAME='problem_build_objects' THEN
  IF TG_OP<>'INSERT' THEN old_ids=ARRAY[OLD.object_id]; END IF;
  IF TG_OP<>'DELETE' THEN
   new_ids=ARRAY[NEW.object_id]; SELECT organization_id INTO owner_org FROM problem_builds WHERE id=NEW.build_id;
   is_output=NEW.role='output';
  END IF;
 ELSIF TG_TABLE_NAME='problem_packages' THEN
  IF TG_OP<>'INSERT' THEN old_ids=ARRAY[OLD.object_id]; END IF;
  IF TG_OP<>'DELETE' THEN new_ids=ARRAY[NEW.object_id];owner_org=NEW.organization_id; END IF;
 ELSIF TG_TABLE_NAME='request_upload_objects' THEN
  IF TG_OP<>'INSERT' THEN old_ids=ARRAY[OLD.object_id]; END IF;
  IF TG_OP<>'DELETE' THEN new_ids=ARRAY[NEW.object_id];owner_org=NEW.organization_id;is_output=true; END IF;
 ELSIF TG_TABLE_NAME='request_idempotency' THEN
  IF TG_OP<>'INSERT' THEN old_ids=ARRAY[OLD.upload_id]; END IF;
  IF TG_OP<>'DELETE' THEN new_ids=ARRAY[NEW.upload_id];owner_org=NEW.organization_id;is_output=true; END IF;
 END IF;
 SELECT COALESCE(array_agg(DISTINCT id ORDER BY id),'{}') INTO ids FROM unnest(old_ids||new_ids) x(id) WHERE id IS NOT NULL;
 FOREACH oid IN ARRAY ids LOOP
  PERFORM 1 FROM stored_objects WHERE id=oid FOR UPDATE;
 END LOOP;
 -- These are separate statements after all waits have completed.
 FOREACH oid IN ARRAY new_ids LOOP
  IF oid IS NULL THEN CONTINUE; END IF;
  SELECT * INTO obj FROM stored_objects WHERE id=oid;
  IF NOT FOUND OR obj.organization_id<>owner_org OR obj.state NOT IN ('available','uploading')
   OR (NOT is_output AND obj.state<>'available') THEN RAISE EXCEPTION 'object reference is not available in owner organization'; END IF;
 END LOOP;
 FOREACH oid IN ARRAY ids LOOP
  UPDATE stored_objects SET unreferenced_at=CASE WHEN stored_object_has_references(oid) THEN NULL ELSE COALESCE(unreferenced_at,clock_timestamp()) END WHERE id=oid;
 END LOOP;
 RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER request_object_refs AFTER INSERT OR DELETE OR UPDATE ON request_idempotency FOR EACH ROW EXECUTE FUNCTION maintain_stored_object_references();
CREATE TRIGGER upload_object_refs AFTER INSERT OR DELETE OR UPDATE ON request_upload_objects FOR EACH ROW EXECUTE FUNCTION maintain_stored_object_references();
CREATE TRIGGER draft_asset_object_refs AFTER INSERT OR DELETE OR UPDATE ON problem_draft_assets FOR EACH ROW EXECUTE FUNCTION maintain_stored_object_references();
CREATE TRIGGER draft_test_object_refs AFTER INSERT OR DELETE OR UPDATE ON problem_draft_tests FOR EACH ROW EXECUTE FUNCTION maintain_stored_object_references();
CREATE TRIGGER build_object_refs AFTER INSERT OR DELETE OR UPDATE ON problem_build_objects FOR EACH ROW EXECUTE FUNCTION maintain_stored_object_references();
CREATE TRIGGER package_object_refs AFTER INSERT OR DELETE OR UPDATE ON problem_packages FOR EACH ROW EXECUTE FUNCTION maintain_stored_object_references();
