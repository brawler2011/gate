-- +goose Up
ALTER TABLE problem_packages
    ADD COLUMN title text NOT NULL DEFAULT '',
    ADD COLUMN short_name text NOT NULL DEFAULT '',
    ADD COLUMN time_limit_ms integer NOT NULL DEFAULT 1000,
    ADD COLUMN memory_limit_mb integer NOT NULL DEFAULT 256;
UPDATE problem_packages pp SET title=p.title, short_name=p.short_name,
    time_limit_ms=p.time_limit_ms, memory_limit_mb=p.memory_limit_mb
FROM problems p WHERE p.id=pp.problem_id;

-- Catalog metadata is captured once, when an immutable package is published.
-- +goose StatementBegin
CREATE FUNCTION capture_package_metadata() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    SELECT p.title,p.short_name,p.time_limit_ms,p.memory_limit_mb
    INTO NEW.title,NEW.short_name,NEW.time_limit_ms,NEW.memory_limit_mb
    FROM problems p WHERE p.id=NEW.problem_id;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER package_metadata BEFORE INSERT ON problem_packages
FOR EACH ROW EXECUTE FUNCTION capture_package_metadata();

ALTER TABLE submissions ADD COLUMN package_id uuid REFERENCES problem_packages(id);
UPDATE submissions s SET package_id=COALESCE(
    (SELECT j.package_id FROM judge_jobs j WHERE j.submission_id=s.id ORDER BY j.generation LIMIT 1),
    (SELECT cp.package_id FROM contest_problems cp WHERE cp.contest_id=s.contest_id AND cp.problem_id=s.problem_id));

-- Keep published snapshots while any contest or submission references them.
ALTER TABLE contest_problems DROP CONSTRAINT contest_problems_problem_id_fkey;
ALTER TABLE contest_problems ADD CONSTRAINT contest_problems_problem_id_fkey FOREIGN KEY(problem_id) REFERENCES problems(id) ON DELETE RESTRICT;

-- +goose Down
ALTER TABLE contest_problems DROP CONSTRAINT contest_problems_problem_id_fkey;
ALTER TABLE contest_problems ADD CONSTRAINT contest_problems_problem_id_fkey FOREIGN KEY(problem_id) REFERENCES problems(id) ON DELETE CASCADE;
ALTER TABLE submissions DROP COLUMN package_id;
DROP TRIGGER package_metadata ON problem_packages;
DROP FUNCTION capture_package_metadata();
ALTER TABLE problem_packages DROP COLUMN title, DROP COLUMN short_name, DROP COLUMN time_limit_ms, DROP COLUMN memory_limit_mb;
