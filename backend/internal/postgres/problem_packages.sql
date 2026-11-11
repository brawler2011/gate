-- Problem Packages queries

-- name: GetProblemPackageByID :one
SELECT * FROM problem_packages WHERE id = $1;

-- name: ListProblemPackages :many
SELECT * FROM problem_packages
WHERE problem_id = $1
ORDER BY revision DESC,created_at DESC,id DESC
LIMIT sqlc.arg('page_size')::int OFFSET sqlc.arg('page_offset')::bigint;

-- name: GetReadyPackage :one
SELECT * FROM problem_packages
WHERE problem_id = $1
ORDER BY revision DESC,created_at DESC,id DESC
LIMIT 1;

-- name: CountProblemPackages :one
SELECT COUNT(*) FROM problem_packages
WHERE problem_id = $1;
