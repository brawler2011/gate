-- Lock-then-recheck queries use separate statements so each sees a fresh snapshot.

-- name: DeleteUnreferencedObjects :many
-- A reference is only ever copied from a committed one, so any statement snapshot sees every live object referenced.
DELETE FROM stored_objects WHERE id IN (
 SELECT o.id FROM stored_objects o
 WHERE NOT EXISTS(SELECT 1 FROM problem_drafts d WHERE o.id=ANY(d.object_ids))
  AND NOT EXISTS(SELECT 1 FROM problem_builds b WHERE b.snapshot->'input_objects' @> jsonb_build_array(o.id))
  AND NOT EXISTS(SELECT 1 FROM problem_packages p WHERE p.object_id=o.id)
  AND NOT EXISTS(SELECT 1 FROM problem_package_assets a WHERE a.object_id=o.id)
  AND NOT EXISTS(SELECT 1 FROM problem_package_pdfs f WHERE f.object_id=o.id)
 ORDER BY o.id LIMIT 100)
RETURNING key;

-- name: ListStoredObjectKeys :many
SELECT key FROM stored_objects WHERE key=ANY(sqlc.arg(keys)::text[]);

-- name: InsertStoredObject :exec
INSERT INTO stored_objects(id,organization_id,purpose,key,sha256,size)VALUES($1,$2,$3,$4,$5,$6);

-- name: ListPackageIDsPastRetention :many
SELECT id FROM problem_packages
WHERE created_at<=clock_timestamp()-sqlc.arg(retention_ms)::bigint*interval '1 millisecond' ORDER BY id LIMIT 100;

-- name: PackageBuildID :one
SELECT build_id FROM problem_packages WHERE id=$1;

-- name: LockProblemPackage :one
SELECT 1::int AS locked FROM problem_packages WHERE id=$1 FOR UPDATE;

-- name: PackagePruneEligible :one
SELECT (created_at<=clock_timestamp()-sqlc.arg(retention_ms)::bigint*interval '1 millisecond'
 AND NOT EXISTS(SELECT 1 FROM contest_problems cp WHERE cp.package_id=sqlc.arg(id))
 AND NOT EXISTS(SELECT 1 FROM submissions s WHERE s.package_id=sqlc.arg(id))
 AND NOT EXISTS(SELECT 1 FROM judge_jobs j WHERE j.package_id=sqlc.arg(id)))::boolean AS eligible
FROM problem_packages WHERE id=sqlc.arg(id);

-- name: DeleteProblemPackage :exec
DELETE FROM problem_packages WHERE id=$1;

-- name: ListBuildIDsPastRetention :many
SELECT id FROM problem_builds
WHERE state IN ('succeeded','failed','cancelled') AND finished_at<=clock_timestamp()-sqlc.arg(retention_ms)::bigint*interval '1 millisecond'
 AND NOT EXISTS(SELECT 1 FROM problem_packages WHERE build_id=problem_builds.id) ORDER BY id LIMIT 100;

-- name: BuildPruneEligible :one
SELECT (state IN ('succeeded','failed','cancelled') AND finished_at<=clock_timestamp()-sqlc.arg(retention_ms)::bigint*interval '1 millisecond'
 AND NOT EXISTS(SELECT 1 FROM problem_packages WHERE build_id=sqlc.arg(id)))::boolean AS eligible
FROM problem_builds WHERE id=sqlc.arg(id);

-- name: DeleteProblemBuild :exec
DELETE FROM problem_builds WHERE id=$1;

-- name: BuildAttemptActive :one
SELECT EXISTS(SELECT 1 FROM problem_builds WHERE id=$1 AND state='running' AND lease_token=$2 AND lease_deadline>clock_timestamp())::boolean AS active;

-- name: ListStoredObjects :many
SELECT * FROM stored_objects WHERE id=ANY(sqlc.arg(ids)::uuid[]) ORDER BY id;

-- name: SucceededBuildPackageID :one
SELECT p.id FROM problem_packages p JOIN problem_builds b ON b.id=p.build_id WHERE b.id=$1 AND b.state='succeeded';

-- name: BuildPackageID :one
SELECT id FROM problem_packages WHERE build_id=$1;

-- name: InsertProblemPackage :exec
INSERT INTO problem_packages(id,problem_id,organization_id,build_id,object_id,package_hash,revision,
 title,short_name,type,time_limit_ms,memory_limit_mb,input_file,output_file,statements,samples)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16);

-- name: InsertPackageAssets :exec
INSERT INTO problem_package_assets(package_id,name,object_id)
SELECT sqlc.arg(package_id)::uuid,a.name,a.object_id
FROM jsonb_to_recordset(sqlc.arg(assets)::jsonb) a(name text,object_id uuid);

-- name: InsertPackagePDFs :exec
INSERT INTO problem_package_pdfs(package_id,language,object_id)
SELECT sqlc.arg(package_id)::uuid,f.language,f.object_id
FROM jsonb_to_recordset(sqlc.arg(pdfs)::jsonb) f(language text,object_id uuid);

-- name: FindPackagePDF :one
SELECT o.key FROM problem_package_pdfs f JOIN stored_objects o ON o.id=f.object_id
WHERE f.package_id=$1 AND f.language=$2;

-- name: ReadPackageStatement :one
SELECT p.problem_id,p.title,p.samples,p.statements,
 COALESCE((SELECT jsonb_object_agg(a.name,o.sha256) FROM problem_package_assets a JOIN stored_objects o ON o.id=a.object_id WHERE a.package_id=p.id),'{}')::jsonb AS assets
FROM problem_packages p WHERE p.id=$1;

-- name: ListPackageAssetsBySHA256 :many
SELECT a.name,o.key FROM problem_package_assets a JOIN stored_objects o ON o.id=a.object_id
WHERE a.package_id=$1 AND o.sha256=$2 ORDER BY a.name;

-- name: MarkBuildSucceeded :execrows
UPDATE problem_builds SET state='succeeded',lease_token=NULL,lease_deadline=NULL,finished_at=clock_timestamp()
WHERE id=$1 AND state='running' AND lease_token=$2 AND lease_deadline>clock_timestamp();

-- name: ReadPackageDescriptor :one
SELECT id,problem_id,object_id FROM problem_packages WHERE id=$1;

-- name: ProblemOrganizationID :one
SELECT organization_id FROM problems WHERE id=$1;

-- name: InsertProblemBuild :exec
INSERT INTO problem_builds(id,problem_id,organization_id,revision,snapshot,state,max_attempts,deadline)
VALUES(sqlc.arg(id),sqlc.arg(problem_id),sqlc.arg(organization_id),sqlc.arg(revision),sqlc.arg(snapshot)::jsonb,
 'queued',sqlc.arg(max_attempts),clock_timestamp()+sqlc.arg(duration_ms)::bigint*interval '1 millisecond');

-- name: BuildExhausted :one
SELECT (deadline<=clock_timestamp() OR attempt>=max_attempts)::boolean AS exhausted FROM problem_builds WHERE id=$1;

-- name: FailExhaustedBuild :exec
UPDATE problem_builds SET state='failed',finished_at=clock_timestamp(),
 diagnostic='{"stage":"queue","kind":"infrastructure","message":"build deadline or attempts exhausted"}' WHERE id=$1;

-- name: StartBuildAttempt :exec
UPDATE problem_builds SET state='running',attempt=attempt+1,lease_token=sqlc.arg(token),
 lease_deadline=LEAST(deadline,clock_timestamp()+sqlc.arg(lease_ms)::bigint*interval '1 millisecond') WHERE id=sqlc.arg(id);

-- name: RenewBuildLease :execrows
UPDATE problem_builds SET lease_deadline=LEAST(deadline,clock_timestamp()+sqlc.arg(lease_ms)::bigint*interval '1 millisecond')
WHERE id=sqlc.arg(id) AND state='running' AND lease_token=sqlc.arg(token) AND lease_deadline>clock_timestamp() AND deadline>clock_timestamp();

-- name: FinishBuildAttemptRow :exec
UPDATE problem_builds SET state=CASE WHEN sqlc.arg(retry)::boolean AND attempt<max_attempts AND deadline>clock_timestamp() THEN 'queued' ELSE 'failed' END,
 finished_at=CASE WHEN sqlc.arg(retry)::boolean AND attempt<max_attempts AND deadline>clock_timestamp() THEN NULL ELSE clock_timestamp() END,
 lease_token=NULL,lease_deadline=NULL,diagnostic=sqlc.arg(diagnostic)::jsonb WHERE id=sqlc.arg(id) AND lease_token=sqlc.arg(token);

-- name: RecordBuildSolutions :execrows
UPDATE problem_builds SET solutions=sqlc.arg(solutions)::jsonb
WHERE id=sqlc.arg(id) AND state='running' AND lease_token=sqlc.arg(token) AND lease_deadline>clock_timestamp();

-- name: RecordBuildProgramTests :execrows
UPDATE problem_builds SET program_tests=sqlc.arg(program_tests)::jsonb
WHERE id=sqlc.arg(id) AND state='running' AND lease_token=sqlc.arg(token) AND lease_deadline>clock_timestamp();

-- name: ListRecoverableBuildIDs :many
SELECT id FROM problem_builds WHERE state='running' AND (lease_deadline<=clock_timestamp() OR deadline<=clock_timestamp())
ORDER BY id LIMIT 100;

-- name: BuildAttemptExpired :one
SELECT (lease_deadline<=clock_timestamp() OR deadline<=clock_timestamp())::boolean AS expired
FROM problem_builds WHERE id=$1;

-- name: RequeueExpiredBuild :exec
UPDATE problem_builds SET state=CASE WHEN attempt<max_attempts AND deadline>clock_timestamp() THEN 'queued' ELSE 'failed' END,
 finished_at=CASE WHEN attempt<max_attempts AND deadline>clock_timestamp() THEN NULL ELSE clock_timestamp() END,
 lease_token=NULL,lease_deadline=NULL,diagnostic='{"stage":"recovery","kind":"infrastructure","message":"attempt expired"}' WHERE id=$1;

-- name: CancelBuildRow :exec
UPDATE problem_builds SET state='cancelled',lease_token=NULL,lease_deadline=NULL,finished_at=clock_timestamp() WHERE id=$1;

-- name: ListProblemBuildIDs :many
SELECT id FROM problem_builds WHERE problem_id=$1 ORDER BY created_at DESC,id DESC LIMIT 100;

-- name: UpdateProblemDraftRow :exec
UPDATE problem_drafts SET updated_at=clock_timestamp(),revision=$2,title=$3,short_name=$4,type=$5,time_limit_ms=$6,memory_limit_mb=$7,max_score=$8,
 content=$9,object_ids=$10 WHERE problem_id=$1;

