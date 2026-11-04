-- Problem Packages queries

-- name: CreateProblemPackage :one
INSERT INTO problem_packages (id, problem_id, organization_id, package_hash, status, version, compiled_at)
VALUES ($1, $2, $3, $4, $5, (SELECT COALESCE(MAX(version), 0) + 1 FROM problem_packages WHERE problem_id = $2), CASE WHEN $5::package_status = 'ready'::package_status THEN NOW() ELSE NULL END)
RETURNING *;

-- name: GetProblemPackageByID :one
SELECT * FROM problem_packages WHERE id = $1;

-- name: GetProblemPackageByVersion :one
SELECT * FROM problem_packages WHERE problem_id = $1 AND version = $2;

-- name: ListProblemPackages :many
SELECT * FROM problem_packages
WHERE problem_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetReadyPackage :one
SELECT * FROM problem_packages
WHERE problem_id = $1 AND status = 'ready'
ORDER BY created_at DESC
LIMIT 1;
