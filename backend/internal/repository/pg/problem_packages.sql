-- Problem Packages queries

-- name: CreateProblemPackage :one
INSERT INTO problem_packages (id, problem_id, organization_id, package_hash, status, version)
VALUES ($1, $2, $3, $4, $5, (SELECT COALESCE(MAX(version), 0) + 1 FROM problem_packages WHERE problem_id = $2))
RETURNING *;

-- name: GetProblemPackageByID :one
SELECT * FROM problem_packages WHERE id = $1;

-- name: GetProblemPackageByHash :one
SELECT * FROM problem_packages WHERE package_hash = $1;

-- name: GetProblemPackageByVersion :one
SELECT * FROM problem_packages WHERE problem_id = $1 AND version = $2;

-- name: ListProblemPackages :many
SELECT * FROM problem_packages
WHERE problem_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdatePackageStatus :exec
UPDATE problem_packages
SET status = $2,
    url = COALESCE(sqlc.narg('url'), url),
    build_log = COALESCE(sqlc.narg('build_log'), build_log),
    compiled_at = CASE WHEN $2 = 'ready'::package_status THEN NOW() ELSE compiled_at END
WHERE id = $1;

-- name: DeleteProblemPackage :exec
DELETE FROM problem_packages WHERE id = $1;

-- name: GetReadyPackage :one
SELECT * FROM problem_packages
WHERE problem_id = $1 AND status = 'ready'
ORDER BY created_at DESC
LIMIT 1;
