-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS images (
    id uuid primary key,
    image text not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);
CREATE INDEX IF NOT EXISTS images_id_idx ON images (id);
CREATE TRIGGER on_images_update BEFORE
UPDATE ON images FOR EACH ROW EXECUTE FUNCTION updated_at_update();
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS images_id_idx;
DROP TABLE IF EXISTS images;
-- +goose StatementEnd