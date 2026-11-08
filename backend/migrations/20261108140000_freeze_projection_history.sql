-- +goose Up
CREATE TABLE submission_state_history (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    submission_id uuid NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
    observed_at timestamptz NOT NULL,
    state integer NOT NULL
);
CREATE INDEX submission_state_history_lookup ON submission_state_history(submission_id, observed_at DESC, id DESC);
-- Existing history is unknown: do not invent pre-freeze results.
INSERT INTO submission_state_history(submission_id,observed_at,state) SELECT id,clock_timestamp(),state FROM submissions;
-- +goose StatementBegin
CREATE FUNCTION capture_submission_state() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  INSERT INTO submission_state_history(submission_id,observed_at,state) VALUES(NEW.id,clock_timestamp(),NEW.state);
  RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER submission_state_changed AFTER INSERT OR UPDATE OF state ON submissions FOR EACH ROW EXECUTE FUNCTION capture_submission_state();

-- +goose StatementBegin
CREATE FUNCTION capture_manual_freeze() RETURNS trigger LANGUAGE plpgsql AS $$
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
UPDATE contests SET settings=jsonb_set(settings,'{freeze_started_at}',to_jsonb(clock_timestamp())) WHERE settings->>'freeze_status'='frozen';
CREATE TRIGGER manual_freeze_changed BEFORE INSERT OR UPDATE OF settings ON contests FOR EACH ROW EXECUTE FUNCTION capture_manual_freeze();
-- +goose Down
DROP TRIGGER manual_freeze_changed ON contests;
DROP FUNCTION capture_manual_freeze();
DROP TRIGGER submission_state_changed ON submissions;
DROP FUNCTION capture_submission_state();
DROP TABLE submission_state_history;
