-- +goose Up
-- A conservative shared revision protects contest commands while dependency-
-- specific revisions are not yet available. Readers never lock this row.
CREATE TABLE access_revision (singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton), revision bigint NOT NULL DEFAULT 1);
INSERT INTO access_revision DEFAULT VALUES;
-- +goose StatementBegin
CREATE FUNCTION bump_access_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    UPDATE access_revision SET revision=revision+1;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd
-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['users','organizations','organization_members','teams','team_members','problem_members','problem_teams','contest_members','contest_teams','problems','contests','problem_packages','contest_problems','contest_user_problem_blocks'] LOOP
    EXECUTE format('CREATE TRIGGER access_revision_changed AFTER INSERT OR UPDATE OR DELETE ON %I FOR EACH STATEMENT EXECUTE FUNCTION bump_access_revision()',t);
  END LOOP;
END;
$$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['users','organizations','organization_members','teams','team_members','problem_members','problem_teams','contest_members','contest_teams','problems','contests','problem_packages','contest_problems','contest_user_problem_blocks'] LOOP
    EXECUTE format('DROP TRIGGER access_revision_changed ON %I',t);
  END LOOP;
END;
$$;
-- +goose StatementEnd
DROP FUNCTION bump_access_revision();
DROP TABLE access_revision;
