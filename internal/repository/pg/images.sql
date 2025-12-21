-- name: CreateImage :exec
INSERT INTO images (
        id,
        image
    )
VALUES (
        @id::uuid,
        @image
    );

-- name: GetImageById :one
SELECT *
FROM images
WHERE id = @id::uuid
LIMIT 1;

-- name: DeleteImage :exec
DELETE FROM images
WHERE id = @id::uuid;


