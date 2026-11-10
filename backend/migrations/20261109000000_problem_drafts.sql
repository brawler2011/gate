-- +goose Up
-- Task data uses a new format; no legacy package/data migration is supported.
TRUNCATE problems CASCADE;
ALTER TABLE submissions ALTER COLUMN score TYPE numeric(12,2);
ALTER TABLE problems ADD CONSTRAINT problems_id_organization_unique UNIQUE(id, organization_id);
-- The workspace ZIP pointer is replaced by drafts and builds.
ALTER TABLE problems DROP COLUMN workspace_hash;

CREATE TABLE stored_objects (
    id uuid PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
    purpose text NOT NULL CHECK(purpose IN ('test-input','test-answer','asset','package')),
    key text NOT NULL UNIQUE,
    sha256 text NOT NULL CHECK(sha256 ~ '^[0-9a-f]{64}$'),
    size bigint NOT NULL CHECK(size >= 0),
    state text NOT NULL CHECK(state IN ('uploading','available','deleting','deleted')),
    upload_deadline timestamptz NOT NULL,
    unreferenced_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE(id, organization_id),
    CHECK(key = 'objects/' || organization_id::text || '/' || id::text)
);
CREATE INDEX stored_objects_gc ON stored_objects(state, unreferenced_at);

CREATE TABLE problem_drafts (
    problem_id uuid PRIMARY KEY,
    organization_id uuid NOT NULL,
    revision bigint NOT NULL DEFAULT 1 CHECK(revision > 0),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    title text NOT NULL CHECK(octet_length(title) <= 1024),
    short_name text NOT NULL CHECK(octet_length(short_name) BETWEEN 1 AND 128),
    type text NOT NULL CHECK(type IN ('pass-fail','scoring','interactive')),
    time_limit_ms integer NOT NULL CHECK(time_limit_ms > 0),
    memory_limit_mb integer NOT NULL CHECK(memory_limit_mb > 0),
    max_score numeric(12,2) CHECK(max_score >= 0),
    checker_id uuid,
    validator_id uuid,
    interactor_id uuid,
    main_solution_id uuid,
    CHECK((type='scoring') = (max_score IS NOT NULL)),
    FOREIGN KEY(problem_id,organization_id) REFERENCES problems(id,organization_id) ON DELETE CASCADE,
    UNIQUE(organization_id,short_name)
);
CREATE INDEX problem_drafts_title_trgm ON problem_drafts USING GIN (title gin_trgm_ops);
-- Title, short name and limits belong to the draft only; the problem row is identity and access.
ALTER TABLE problems DROP COLUMN title, DROP COLUMN short_name, DROP COLUMN time_limit_ms, DROP COLUMN memory_limit_mb;

CREATE VIEW problem_catalog AS
 SELECT p.id,p.organization_id,p.owner_id,p.visibility,p.is_template,p.created_at,d.updated_at,
 d.title,d.short_name,d.time_limit_ms,d.memory_limit_mb
 FROM problems p JOIN problem_drafts d ON d.problem_id=p.id;

-- +goose StatementBegin
CREATE FUNCTION valid_draft_path(path text) RETURNS boolean LANGUAGE sql IMMUTABLE AS $$
    SELECT path <> '' AND path <> '.' AND path <> 'problem.yaml'
      AND path <> '.gate' AND path NOT LIKE '.gate/%'
      AND octet_length(path) <= 512 AND path !~ '(^/|/$|//|(^|/)\.{1,2}(/|$))'
      AND position(chr(92) in path) = 0;
$$;
-- +goose StatementEnd

CREATE TABLE problem_draft_files (
    problem_id uuid NOT NULL REFERENCES problem_drafts(problem_id) ON DELETE CASCADE,
    path text NOT NULL CHECK(valid_draft_path(path)),
    content text NOT NULL,
    PRIMARY KEY(problem_id,path)
);
CREATE TABLE problem_draft_components (
    problem_id uuid NOT NULL REFERENCES problem_drafts(problem_id) ON DELETE CASCADE,
    id uuid NOT NULL,
    role text NOT NULL CHECK(role IN ('checker','validator','interactor','generator','solution')),
    path text NOT NULL,
    language text NOT NULL CHECK(language <> ''),
    dependencies jsonb NOT NULL CHECK(jsonb_typeof(dependencies)='object'),
    PRIMARY KEY(problem_id,id),
    UNIQUE(problem_id,id,role),
    FOREIGN KEY(problem_id,path) REFERENCES problem_draft_files(problem_id,path) DEFERRABLE INITIALLY DEFERRED
);
ALTER TABLE problem_drafts
    ADD FOREIGN KEY(problem_id,checker_id) REFERENCES problem_draft_components(problem_id,id) DEFERRABLE INITIALLY DEFERRED,
    ADD FOREIGN KEY(problem_id,validator_id) REFERENCES problem_draft_components(problem_id,id) DEFERRABLE INITIALLY DEFERRED,
    ADD FOREIGN KEY(problem_id,interactor_id) REFERENCES problem_draft_components(problem_id,id) DEFERRABLE INITIALLY DEFERRED,
    ADD FOREIGN KEY(problem_id,main_solution_id) REFERENCES problem_draft_components(problem_id,id) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE problem_draft_subtasks (
    problem_id uuid NOT NULL REFERENCES problem_drafts(problem_id) ON DELETE CASCADE,
    id uuid NOT NULL,
    ordinal integer NOT NULL CHECK(ordinal > 0),
    name text NOT NULL,
    points numeric(12,2) NOT NULL CHECK(points >= 0),
    policy text NOT NULL CHECK(policy IN ('complete','each')),
    PRIMARY KEY(problem_id,id),
    UNIQUE(problem_id,ordinal) DEFERRABLE INITIALLY DEFERRED
);
CREATE TABLE problem_draft_subtask_dependencies (
    problem_id uuid NOT NULL,
    subtask_id uuid NOT NULL,
    depends_on uuid NOT NULL,
    PRIMARY KEY(problem_id,subtask_id,depends_on),
    CHECK(subtask_id <> depends_on),
    FOREIGN KEY(problem_id,subtask_id) REFERENCES problem_draft_subtasks(problem_id,id) ON DELETE CASCADE,
    FOREIGN KEY(problem_id,depends_on) REFERENCES problem_draft_subtasks(problem_id,id) DEFERRABLE INITIALLY DEFERRED
);
CREATE TABLE problem_draft_tests (
    problem_id uuid NOT NULL REFERENCES problem_drafts(problem_id) ON DELETE CASCADE,
    id uuid NOT NULL,
    ordinal integer NOT NULL CHECK(ordinal > 0),
    subtask_id uuid NOT NULL,
    sample boolean NOT NULL,
    points numeric(12,2) NOT NULL CHECK(points >= 0),
    input_mode text NOT NULL CHECK(input_mode IN ('manual','generator')),
    input_object_id uuid REFERENCES stored_objects(id) ON DELETE RESTRICT,
    generator_id uuid,
    generator_args text[] NOT NULL,
    seed bigint,
    answer_mode text NOT NULL CHECK(answer_mode IN ('provided','generated','interactive')),
    answer_object_id uuid REFERENCES stored_objects(id) ON DELETE RESTRICT,
    solution_id uuid,
    PRIMARY KEY(problem_id,id),
    UNIQUE(problem_id,ordinal) DEFERRABLE INITIALLY DEFERRED,
    FOREIGN KEY(problem_id,subtask_id) REFERENCES problem_draft_subtasks(problem_id,id) DEFERRABLE INITIALLY DEFERRED,
    FOREIGN KEY(problem_id,generator_id) REFERENCES problem_draft_components(problem_id,id) DEFERRABLE INITIALLY DEFERRED,
    FOREIGN KEY(problem_id,solution_id) REFERENCES problem_draft_components(problem_id,id) DEFERRABLE INITIALLY DEFERRED,
    CHECK((input_mode='manual' AND input_object_id IS NOT NULL AND generator_id IS NULL AND seed IS NULL AND cardinality(generator_args)=0)
       OR (input_mode='generator' AND input_object_id IS NULL AND generator_id IS NOT NULL AND seed IS NOT NULL)),
    CHECK((answer_mode='provided' AND answer_object_id IS NOT NULL AND solution_id IS NULL)
       OR (answer_mode='generated' AND answer_object_id IS NULL AND solution_id IS NOT NULL)
       OR (answer_mode='interactive' AND answer_object_id IS NULL AND solution_id IS NULL))
);
CREATE INDEX problem_draft_tests_input ON problem_draft_tests(input_object_id);
CREATE INDEX problem_draft_tests_answer ON problem_draft_tests(answer_object_id);
CREATE TABLE problem_draft_assets (
    problem_id uuid NOT NULL REFERENCES problem_drafts(problem_id) ON DELETE CASCADE,
    path text NOT NULL CHECK(valid_draft_path(path)),
    object_id uuid NOT NULL REFERENCES stored_objects(id) ON DELETE RESTRICT,
    PRIMARY KEY(problem_id,path)
);
CREATE INDEX problem_draft_assets_object ON problem_draft_assets(object_id);

CREATE TABLE problem_builds (
    id uuid PRIMARY KEY,
    problem_id uuid NOT NULL,
    organization_id uuid NOT NULL,
    revision bigint NOT NULL CHECK(revision > 0),
    snapshot jsonb NOT NULL,
    schema_version integer NOT NULL CHECK(schema_version=1),
    toolchain_digest text NOT NULL CHECK(toolchain_digest ~ '^sha256:[0-9a-f]{64}$'),
    state text NOT NULL CHECK(state IN ('queued','running','succeeded','failed','cancelled')),
    lease_token uuid,
    lease_deadline timestamptz,
    attempt integer NOT NULL DEFAULT 0 CHECK(attempt >= 0),
    max_attempts integer NOT NULL CHECK(max_attempts > 0),
    deadline timestamptz NOT NULL,
    diagnostic jsonb,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    finished_at timestamptz,
    FOREIGN KEY(problem_id,organization_id) REFERENCES problems(id,organization_id) ON DELETE RESTRICT,
    UNIQUE(id,problem_id,organization_id),
    CHECK((state='running')=(lease_token IS NOT NULL AND lease_deadline IS NOT NULL)),
    CHECK((state IN ('succeeded','failed','cancelled'))=(finished_at IS NOT NULL)),
    CHECK(jsonb_typeof(snapshot)='object'),
    CHECK(octet_length(diagnostic::text) <= 131072)
);
CREATE INDEX problem_builds_queue ON problem_builds(created_at,id) WHERE state='queued';
CREATE INDEX problem_builds_recovery ON problem_builds(lease_deadline) WHERE state='running';
CREATE TABLE problem_build_objects (
    build_id uuid NOT NULL REFERENCES problem_builds(id) ON DELETE CASCADE,
    object_id uuid NOT NULL REFERENCES stored_objects(id) ON DELETE RESTRICT,
    role text NOT NULL CHECK(role IN ('input','output')),
    attempt_token uuid,
    PRIMARY KEY(build_id,object_id,role),
    CHECK((role='input' AND attempt_token IS NULL) OR (role='output' AND attempt_token IS NOT NULL))
);
CREATE INDEX problem_build_objects_object ON problem_build_objects(object_id);

CREATE TABLE request_idempotency (
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
    operation text NOT NULL,
    key text NOT NULL CHECK(octet_length(key) BETWEEN 1 AND 200),
    fingerprint text NOT NULL CHECK(fingerprint ~ '^[0-9a-f]{64}$'),
    token uuid NOT NULL,
    lease_deadline timestamptz NOT NULL,
    upload_id uuid REFERENCES stored_objects(id) ON DELETE RESTRICT,
    result jsonb,
    http_status integer CHECK(http_status BETWEEN 200 AND 299),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY(organization_id,operation,key),
    CHECK((result IS NULL)=(http_status IS NULL))
);

CREATE TABLE request_upload_objects (
 organization_id uuid NOT NULL,
 operation text NOT NULL,
 key text NOT NULL,
 ordinal integer NOT NULL CHECK(ordinal>0),
 object_id uuid NOT NULL REFERENCES stored_objects(id) ON DELETE RESTRICT,
 PRIMARY KEY(organization_id,operation,key,ordinal),
 UNIQUE(object_id),
 FOREIGN KEY(organization_id,operation,key) REFERENCES request_idempotency(organization_id,operation,key) ON DELETE RESTRICT
);

ALTER TABLE problem_packages ADD COLUMN build_id uuid NOT NULL UNIQUE REFERENCES problem_builds(id) ON DELETE RESTRICT,
    ADD COLUMN object_id uuid NOT NULL REFERENCES stored_objects(id) ON DELETE RESTRICT;
ALTER TABLE problem_packages ADD COLUMN revision bigint NOT NULL CHECK(revision>0),
 DROP COLUMN version, DROP COLUMN url, DROP COLUMN status, DROP COLUMN build_log, DROP COLUMN compiled_at;
ALTER TABLE problem_packages DROP CONSTRAINT problem_packages_package_hash_key;
CREATE INDEX problem_packages_object ON problem_packages(object_id);

-- +goose StatementBegin
CREATE FUNCTION stored_object_has_references(oid uuid) RETURNS boolean LANGUAGE sql VOLATILE AS $$
 SELECT EXISTS(SELECT 1 FROM problem_draft_tests WHERE input_object_id=oid OR answer_object_id=oid)
 OR EXISTS(SELECT 1 FROM problem_draft_assets WHERE object_id=oid)
 OR EXISTS(SELECT 1 FROM problem_build_objects WHERE object_id=oid)
 OR EXISTS(SELECT 1 FROM problem_packages WHERE object_id=oid)
 OR EXISTS(SELECT 1 FROM request_idempotency WHERE upload_id=oid)
 OR EXISTS(SELECT 1 FROM request_upload_objects WHERE object_id=oid);
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION guard_stored_object() RETURNS trigger LANGUAGE plpgsql AS $$
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

-- +goose StatementBegin
CREATE FUNCTION guard_problem_build() RETURNS trigger LANGUAGE plpgsql AS $$
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
CREATE TRIGGER problem_build_guard BEFORE UPDATE ON problem_builds FOR EACH ROW EXECUTE FUNCTION guard_problem_build();

-- +goose StatementBegin
CREATE FUNCTION guard_build_input() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.role='input' AND EXISTS(SELECT 1 FROM problem_builds WHERE id=OLD.build_id) THEN
        RAISE EXCEPTION 'build inputs are immutable';
    END IF;
    IF TG_OP='UPDATE' THEN RAISE EXCEPTION 'build object references cannot be updated'; END IF;
    RETURN OLD;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER problem_build_input_guard BEFORE UPDATE OR DELETE ON problem_build_objects FOR EACH ROW EXECUTE FUNCTION guard_build_input();

-- +goose StatementBegin
CREATE FUNCTION check_build_inputs() RETURNS trigger LANGUAGE plpgsql AS $$
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
$$;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER problem_build_inputs_snapshot AFTER INSERT OR UPDATE ON problem_builds DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_build_inputs();
CREATE CONSTRAINT TRIGGER problem_build_inputs_refs AFTER INSERT OR UPDATE OR DELETE ON problem_build_objects DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_build_inputs();

DROP TRIGGER package_metadata ON problem_packages;
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION capture_package_metadata() RETURNS trigger LANGUAGE plpgsql AS $$
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
CREATE TRIGGER package_metadata BEFORE INSERT ON problem_packages FOR EACH ROW EXECUTE FUNCTION capture_package_metadata();
-- +goose StatementBegin
CREATE FUNCTION guard_package_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'package is immutable'; END;
$$;
-- +goose StatementEnd
CREATE TRIGGER package_immutable BEFORE UPDATE ON problem_packages FOR EACH ROW EXECUTE FUNCTION guard_package_immutable();
-- +goose StatementBegin
CREATE FUNCTION check_build_publication() RETURNS trigger LANGUAGE plpgsql AS $$
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
CREATE CONSTRAINT TRIGGER build_publication AFTER INSERT OR UPDATE ON problem_builds DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_build_publication();
CREATE CONSTRAINT TRIGGER package_publication AFTER INSERT ON problem_packages DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_build_publication();

-- All link writers use the same object-row protocol, including direct SQL.
-- Owner locks are taken by the operation before reaching these triggers.
-- +goose StatementBegin
CREATE FUNCTION maintain_stored_object_references() RETURNS trigger LANGUAGE plpgsql AS $$
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
CREATE TRIGGER draft_test_object_refs AFTER INSERT OR UPDATE OR DELETE ON problem_draft_tests FOR EACH ROW EXECUTE FUNCTION maintain_stored_object_references();
CREATE TRIGGER draft_asset_object_refs AFTER INSERT OR UPDATE OR DELETE ON problem_draft_assets FOR EACH ROW EXECUTE FUNCTION maintain_stored_object_references();
CREATE TRIGGER build_object_refs AFTER INSERT OR UPDATE OR DELETE ON problem_build_objects FOR EACH ROW EXECUTE FUNCTION maintain_stored_object_references();
CREATE TRIGGER package_object_refs AFTER INSERT OR UPDATE OR DELETE ON problem_packages FOR EACH ROW EXECUTE FUNCTION maintain_stored_object_references();
CREATE TRIGGER request_object_refs AFTER INSERT OR UPDATE OR DELETE ON request_idempotency FOR EACH ROW EXECUTE FUNCTION maintain_stored_object_references();

CREATE TRIGGER upload_object_refs AFTER INSERT OR UPDATE OR DELETE ON request_upload_objects FOR EACH ROW EXECUTE FUNCTION maintain_stored_object_references();

-- +goose Down
DROP VIEW problem_catalog;
ALTER TABLE submissions ALTER COLUMN score TYPE integer;
DROP TRIGGER upload_object_refs ON request_upload_objects;
DROP TABLE request_upload_objects;
DROP TRIGGER draft_test_object_refs ON problem_draft_tests;
DROP TRIGGER draft_asset_object_refs ON problem_draft_assets;
DROP TRIGGER build_object_refs ON problem_build_objects;
DROP TRIGGER package_object_refs ON problem_packages;
DROP TRIGGER request_object_refs ON request_idempotency;
DROP FUNCTION maintain_stored_object_references();
DROP TRIGGER package_publication ON problem_packages;
DROP TRIGGER build_publication ON problem_builds;
DROP FUNCTION check_build_publication();
DROP TRIGGER package_immutable ON problem_packages;
DROP FUNCTION guard_package_immutable();
DROP FUNCTION stored_object_has_references(uuid);
ALTER TABLE problem_packages DROP COLUMN build_id, DROP COLUMN object_id;
DROP TABLE request_idempotency;
DROP TABLE problem_build_objects;
DROP TABLE problem_builds;
DROP FUNCTION check_build_inputs();
DROP FUNCTION guard_build_input();
DROP FUNCTION guard_problem_build();
DROP TABLE problem_draft_assets;
DROP TABLE problem_draft_tests;
DROP TABLE problem_draft_subtask_dependencies;
DROP TABLE problem_draft_subtasks;
ALTER TABLE problem_drafts DROP CONSTRAINT problem_drafts_problem_id_checker_id_fkey,
 DROP CONSTRAINT problem_drafts_problem_id_validator_id_fkey,
 DROP CONSTRAINT problem_drafts_problem_id_interactor_id_fkey,
 DROP CONSTRAINT problem_drafts_problem_id_main_solution_id_fkey;
DROP TABLE problem_draft_components;
DROP TABLE problem_draft_files;
DROP TABLE problem_drafts;
DROP FUNCTION valid_draft_path(text);
DROP TABLE stored_objects;
DROP FUNCTION guard_stored_object();
ALTER TABLE problems DROP CONSTRAINT problems_id_organization_unique;
ALTER TABLE problems ADD COLUMN workspace_hash text NOT NULL DEFAULT '',
 ADD COLUMN title text NOT NULL DEFAULT '', ADD COLUMN short_name text NOT NULL DEFAULT '',
 ADD COLUMN time_limit_ms integer NOT NULL DEFAULT 1000, ADD COLUMN memory_limit_mb integer NOT NULL DEFAULT 256;
