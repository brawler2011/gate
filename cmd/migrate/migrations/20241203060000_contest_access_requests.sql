-- +goose Up
-- +goose StatementBegin
CREATE TYPE contest_access_request_status AS ENUM ('pending', 'approved', 'rejected');

CREATE TABLE IF NOT EXISTS contest_access_requests
(
    id         uuid PRIMARY KEY                  DEFAULT uuid_generate_v4(),
    contest_id uuid                     NOT NULL REFERENCES contests (id) ON DELETE CASCADE,
    user_id    uuid                     NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    status     contest_access_request_status NOT NULL DEFAULT 'pending',
    created_at timestamptz              NOT NULL DEFAULT now(),
    updated_at timestamptz              NOT NULL DEFAULT now(),
    UNIQUE (contest_id, user_id)
);

CREATE INDEX IF NOT EXISTS contest_access_requests_contest_id_idx ON contest_access_requests (contest_id);
CREATE INDEX IF NOT EXISTS contest_access_requests_status_idx ON contest_access_requests (contest_id, status);

CREATE TRIGGER on_contest_access_requests_update
    BEFORE UPDATE
    ON contest_access_requests
    FOR EACH ROW
EXECUTE FUNCTION updated_at_update();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS on_contest_access_requests_update ON contest_access_requests;
DROP INDEX IF EXISTS contest_access_requests_status_idx;
DROP INDEX IF EXISTS contest_access_requests_contest_id_idx;
DROP TABLE IF EXISTS contest_access_requests;
DROP TYPE IF EXISTS contest_access_request_status;
-- +goose StatementEnd

