-- +goose Up
-- +goose StatementBegin
ALTER TABLE submissions
ADD COLUMN failed_test integer DEFAULT NULL;

COMMENT ON COLUMN submissions.failed_test IS 'The test number (1-indexed) where the submission failed. NULL for AC submissions.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE submissions
DROP COLUMN IF EXISTS failed_test;
-- +goose StatementEnd

