-- Statements of the object, upload, build and idempotency protocols.
-- Locks are taken by the caller in the documented order; lock-then-recheck
-- statements are deliberately separate queries so each sees a fresh snapshot.

-- name: ListGCObjectIDs :many
SELECT id FROM stored_objects WHERE state='deleting' OR
 (state IN ('uploading','available') AND unreferenced_at<=clock_timestamp()-sqlc.arg(retention_ms)::bigint*interval '1 millisecond')
ORDER BY id LIMIT 100;

-- name: StoredObjectIDByKey :one
SELECT id FROM stored_objects WHERE key=$1;

-- name: ListPackageIDsPastRetention :many
SELECT id FROM problem_packages
WHERE created_at<=clock_timestamp()-sqlc.arg(retention_ms)::bigint*interval '1 millisecond' ORDER BY id LIMIT 100;

-- name: PackageBuildID :one
SELECT build_id FROM problem_packages WHERE id=$1;

-- name: LockPackageObject :one
SELECT object_id FROM problem_packages WHERE id=$1 FOR UPDATE;

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

-- name: ListBuildObjectIDs :many
SELECT DISTINCT object_id FROM problem_build_objects WHERE build_id=$1 ORDER BY object_id;

-- name: DeleteProblemBuild :exec
DELETE FROM problem_builds WHERE id=$1;

-- name: BuildAttemptActive :one
SELECT EXISTS(SELECT 1 FROM problem_builds WHERE id=$1 AND state='running' AND lease_token=$2 AND lease_deadline>clock_timestamp())::boolean AS active;

-- name: ListBuildInputObjects :many
SELECT o.* FROM problem_build_objects bo JOIN stored_objects o ON o.id=bo.object_id
WHERE bo.build_id=$1 AND bo.role='input' ORDER BY o.id;

-- name: BuildLeaseValid :one
SELECT (state='running' AND lease_token=sqlc.arg(token) AND lease_deadline>clock_timestamp() AND deadline>clock_timestamp())::boolean AS valid
FROM problem_builds WHERE id=sqlc.arg(id);

-- name: BuildOutputObjectID :one
SELECT object_id FROM problem_build_objects WHERE build_id=$1 AND role='output' AND attempt_token=$2;

-- name: InsertPackageObject :exec
INSERT INTO stored_objects(id,organization_id,purpose,key,sha256,size,state,upload_deadline,unreferenced_at)
VALUES(sqlc.arg(id),sqlc.arg(organization_id),'package',sqlc.arg(key),sqlc.arg(sha256),sqlc.arg(size),'uploading',
 LEAST(sqlc.arg(build_deadline)::timestamptz,clock_timestamp()+sqlc.arg(ttl_ms)::bigint*interval '1 millisecond'),NULL);

-- name: InsertBuildOutput :exec
INSERT INTO problem_build_objects(build_id,object_id,role,attempt_token)VALUES($1,$2,'output',$3);

-- name: SucceededBuildPackageID :one
SELECT p.id FROM problem_packages p JOIN problem_builds b ON b.id=p.build_id WHERE b.id=$1 AND b.state='succeeded';

-- name: BuildPackageID :one
SELECT id FROM problem_packages WHERE build_id=$1;

-- name: OutputUploadActive :one
SELECT (EXISTS(SELECT 1 FROM problem_build_objects WHERE build_id=sqlc.arg(build_id) AND object_id=sqlc.arg(object_id) AND role='output' AND attempt_token=sqlc.arg(token))
 AND (SELECT upload_deadline>clock_timestamp() FROM stored_objects WHERE id=sqlc.arg(object_id)))::boolean AS active;

-- name: PublishStoredObject :exec
UPDATE stored_objects SET state='available',unreferenced_at=NULL WHERE id=$1;

-- name: InsertProblemPackage :exec
INSERT INTO problem_packages(id,problem_id,organization_id,build_id,object_id,package_hash,revision,samples)
VALUES($1,$2,$3,$4,$5,$6,$7,$8);

-- name: InsertPackageAssets :exec
INSERT INTO problem_package_assets(package_id,path,object_id)
SELECT sqlc.arg(package_id)::uuid,a->>'path',(a->>'object_id')::uuid
FROM problem_builds b,jsonb_array_elements(b.snapshot->'draft'->'assets') a WHERE b.id=sqlc.arg(build_id);

-- name: ListPackageAssetObjectIDs :many
SELECT object_id FROM problem_package_assets WHERE package_id=$1;

-- name: ReadPackageStatement :one
SELECT p.problem_id,p.samples,b.snapshot,
 COALESCE((SELECT jsonb_object_agg(a.path,o.sha256) FROM problem_package_assets a JOIN stored_objects o ON o.id=a.object_id WHERE a.package_id=p.id),'{}')::jsonb AS assets
FROM problem_packages p JOIN problem_builds b ON b.id=p.build_id WHERE p.id=$1;

-- name: ListPackageAssetsBySHA256 :many
SELECT a.path,o.key FROM problem_package_assets a JOIN stored_objects o ON o.id=a.object_id
WHERE a.package_id=$1 AND o.sha256=$2 AND o.state='available' ORDER BY a.path;

-- name: DeleteBuildOutput :exec
DELETE FROM problem_build_objects WHERE build_id=$1 AND object_id=$2 AND role='output' AND attempt_token=$3;

-- name: MarkBuildSucceeded :exec
UPDATE problem_builds SET state='succeeded',lease_token=NULL,lease_deadline=NULL,finished_at=clock_timestamp()
WHERE id=$1 AND state='running' AND lease_token=$2 AND lease_deadline>clock_timestamp();

-- name: ObjectDeletionEligible :one
SELECT (state IN ('uploading','available') AND NOT stored_object_has_references(id)
 AND unreferenced_at<=clock_timestamp()-sqlc.arg(retention_ms)::bigint*interval '1 millisecond'
 AND (state<>'uploading' OR upload_deadline<=clock_timestamp()))::boolean AS eligible
FROM stored_objects WHERE id=sqlc.arg(id);

-- name: MarkObjectDeleting :exec
UPDATE stored_objects SET state='deleting' WHERE id=$1;

-- name: MarkObjectDeleted :execrows
UPDATE stored_objects SET state='deleted' WHERE id=$1 AND state='deleting';

-- name: ReadPackageDescriptor :one
SELECT p.id,p.build_id,p.problem_id,p.object_id,b.snapshot
FROM problem_packages p JOIN problem_builds b ON b.id=p.build_id WHERE p.id=$1;

-- name: ListOperationUploads :many
SELECT object_id FROM request_upload_objects WHERE organization_id=$1 AND operation=$2 AND key=$3 ORDER BY ordinal;

-- name: ClearOperationUploadID :exec
UPDATE request_idempotency SET upload_id=NULL WHERE organization_id=$1 AND operation=$2 AND key=$3;

-- name: DeleteOperationUploads :exec
DELETE FROM request_upload_objects WHERE organization_id=$1 AND operation=$2 AND key=$3;

-- name: UploadObjectActive :one
SELECT (state='uploading' AND upload_deadline>clock_timestamp())::boolean AS active FROM stored_objects WHERE id=$1;

-- name: InsertUploadObject :exec
INSERT INTO stored_objects(id,organization_id,purpose,key,sha256,size,state,upload_deadline,unreferenced_at)
VALUES(sqlc.arg(id),sqlc.arg(organization_id),sqlc.arg(purpose),sqlc.arg(key),sqlc.arg(sha256),sqlc.arg(size),'uploading',
 clock_timestamp()+sqlc.arg(ttl_ms)::bigint*interval '1 millisecond',NULL);

-- name: InsertOperationUpload :exec
INSERT INTO request_upload_objects(organization_id,operation,key,ordinal,object_id)VALUES($1,$2,$3,$4,$5);

-- name: SetOperationUploadID :exec
UPDATE request_idempotency SET upload_id=$4 WHERE organization_id=$1 AND operation=$2 AND key=$3;

-- name: LockOperationForFinalize :one
SELECT token,fingerprint,result,http_status FROM request_idempotency
WHERE organization_id=$1 AND operation=$2 AND key=$3 FOR UPDATE;

-- name: ProblemOrganizationID :one
SELECT organization_id FROM problems WHERE id=$1;

-- name: PublishUploadedObject :exec
UPDATE stored_objects SET state='available' WHERE id=$1;

-- name: PublishUploadedObjectIfActive :execrows
UPDATE stored_objects SET state='available' WHERE id=$1 AND state='uploading' AND upload_deadline>clock_timestamp();

-- name: ObjectIsAvailable :one
SELECT (state='available')::boolean AS available FROM stored_objects WHERE id=$1;

-- name: ListExpiredOperations :many
SELECT organization_id,operation,key FROM request_idempotency
WHERE result IS NULL AND lease_deadline<=clock_timestamp() LIMIT 100;

-- name: LockOperation :one
SELECT 1::int AS locked FROM request_idempotency WHERE organization_id=$1 AND operation=$2 AND key=$3 FOR UPDATE;

-- name: OperationExpired :one
SELECT (result IS NULL AND lease_deadline<=clock_timestamp())::boolean AS expired FROM request_idempotency
WHERE organization_id=$1 AND operation=$2 AND key=$3;

-- name: InsertProblemBuild :exec
INSERT INTO problem_builds(id,problem_id,organization_id,revision,snapshot,schema_version,toolchain_digest,state,max_attempts,deadline)
VALUES(sqlc.arg(id),sqlc.arg(problem_id),sqlc.arg(organization_id),sqlc.arg(revision),sqlc.arg(snapshot)::jsonb,sqlc.arg(schema_version),
 sqlc.arg(toolchain_digest),'queued',sqlc.arg(max_attempts),clock_timestamp()+sqlc.arg(duration_ms)::bigint*interval '1 millisecond');

-- name: InsertBuildInput :exec
INSERT INTO problem_build_objects(build_id,object_id,role)VALUES($1,$2,'input');

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

-- name: ListAttemptOutputObjects :many
SELECT object_id FROM problem_build_objects WHERE build_id=$1 AND role='output' AND attempt_token=$2 ORDER BY object_id;

-- name: DeleteAttemptOutputs :exec
DELETE FROM problem_build_objects WHERE build_id=$1 AND role='output' AND attempt_token=$2;

-- name: FinishBuildAttemptRow :exec
UPDATE problem_builds SET state=CASE WHEN sqlc.arg(retry)::boolean AND attempt<max_attempts AND deadline>clock_timestamp() THEN 'queued' ELSE 'failed' END,
 finished_at=CASE WHEN sqlc.arg(retry)::boolean AND attempt<max_attempts AND deadline>clock_timestamp() THEN NULL ELSE clock_timestamp() END,
 lease_token=NULL,lease_deadline=NULL,diagnostic=sqlc.arg(diagnostic)::jsonb WHERE id=sqlc.arg(id) AND lease_token=sqlc.arg(token);

-- name: ListRecoverableBuildIDs :many
SELECT b.id FROM problem_builds b WHERE b.state='running' AND
 (b.lease_deadline<=clock_timestamp() OR b.deadline<=clock_timestamp() OR EXISTS(
 SELECT 1 FROM problem_build_objects bo JOIN stored_objects o ON o.id=bo.object_id
 WHERE bo.build_id=b.id AND bo.role='output' AND bo.attempt_token=b.lease_token AND o.upload_deadline<=clock_timestamp())) ORDER BY b.id LIMIT 100;

-- name: BuildAttemptExpired :one
SELECT (lease_deadline<=clock_timestamp() OR deadline<=clock_timestamp() OR EXISTS(
 SELECT 1 FROM problem_build_objects bo JOIN stored_objects o ON o.id=bo.object_id
 WHERE bo.build_id=b.id AND bo.role='output' AND bo.attempt_token=b.lease_token AND o.upload_deadline<=clock_timestamp()))::boolean AS expired
FROM problem_builds b WHERE b.id=$1;

-- name: RequeueExpiredBuild :exec
UPDATE problem_builds SET state=CASE WHEN attempt<max_attempts AND deadline>clock_timestamp() THEN 'queued' ELSE 'failed' END,
 finished_at=CASE WHEN attempt<max_attempts AND deadline>clock_timestamp() THEN NULL ELSE clock_timestamp() END,
 lease_token=NULL,lease_deadline=NULL,diagnostic='{"stage":"recovery","kind":"infrastructure","message":"attempt expired"}' WHERE id=$1;

-- name: CancelBuildRow :exec
UPDATE problem_builds SET state='cancelled',lease_token=NULL,lease_deadline=NULL,finished_at=clock_timestamp() WHERE id=$1;

-- name: ListProblemBuildIDs :many
SELECT id FROM problem_builds WHERE problem_id=$1 ORDER BY created_at DESC,id DESC LIMIT 100;

-- name: ReserveOperationRow :exec
INSERT INTO request_idempotency(organization_id,operation,key,fingerprint,token,lease_deadline)
VALUES(sqlc.arg(organization_id),sqlc.arg(operation),sqlc.arg(key),sqlc.arg(fingerprint),sqlc.arg(token),
 clock_timestamp()+sqlc.arg(lease_ms)::bigint*interval '1 millisecond') ON CONFLICT DO NOTHING;

-- name: LockOperationRow :one
SELECT fingerprint,token,result,http_status,(lease_deadline>clock_timestamp())::boolean AS active FROM request_idempotency
WHERE organization_id=$1 AND operation=$2 AND key=$3 FOR UPDATE;

-- name: OperationLeaseActive :one
SELECT (lease_deadline>clock_timestamp())::boolean AS active FROM request_idempotency
WHERE organization_id=$1 AND operation=$2 AND key=$3;

-- name: TakeOverOperation :exec
UPDATE request_idempotency SET token=sqlc.arg(token),lease_deadline=clock_timestamp()+sqlc.arg(lease_ms)::bigint*interval '1 millisecond'
WHERE organization_id=sqlc.arg(organization_id) AND operation=sqlc.arg(operation) AND key=sqlc.arg(key);

-- name: FinishOperationRow :execrows
UPDATE request_idempotency SET result=sqlc.arg(result)::jsonb,http_status=sqlc.arg(http_status),upload_id=NULL
WHERE organization_id=sqlc.arg(organization_id) AND operation=sqlc.arg(operation) AND key=sqlc.arg(key) AND token=sqlc.arg(token)
 AND lease_deadline>clock_timestamp() AND result IS NULL;

-- name: ReadOperationResult :one
SELECT result,http_status FROM request_idempotency
WHERE organization_id=$1 AND operation=$2 AND key=$3 AND fingerprint=$4;

-- name: UpdateProblemDraftRow :exec
UPDATE problem_drafts SET updated_at=clock_timestamp(),revision=$2,title=$3,short_name=$4,type=$5,time_limit_ms=$6,memory_limit_mb=$7,max_score=$8,
 checker_id=$9,validator_id=$10,interactor_id=$11,main_solution_id=$12 WHERE problem_id=$1;

-- name: DeleteDraftTests :exec
DELETE FROM problem_draft_tests WHERE problem_id=$1;
-- name: DeleteDraftAssets :exec
DELETE FROM problem_draft_assets WHERE problem_id=$1;
-- name: DeleteDraftSubtaskDependencies :exec
DELETE FROM problem_draft_subtask_dependencies WHERE problem_id=$1;
-- name: DeleteDraftSubtasks :exec
DELETE FROM problem_draft_subtasks WHERE problem_id=$1;
-- name: DeleteDraftComponents :exec
DELETE FROM problem_draft_components WHERE problem_id=$1;
-- name: DeleteDraftFiles :exec
DELETE FROM problem_draft_files WHERE problem_id=$1;

-- name: InsertDraftFile :exec
INSERT INTO problem_draft_files(problem_id,path,content)VALUES($1,$2,$3);

-- name: InsertDraftComponent :exec
INSERT INTO problem_draft_components(problem_id,id,role,path,language,dependencies)
VALUES(sqlc.arg(problem_id),sqlc.arg(id),sqlc.arg(role),sqlc.arg(path),sqlc.arg(language),sqlc.arg(dependencies)::jsonb);

-- name: InsertDraftSubtask :exec
INSERT INTO problem_draft_subtasks(problem_id,id,ordinal,name,points,policy)VALUES($1,$2,$3,$4,$5,$6);

-- name: InsertDraftSubtaskDependency :exec
INSERT INTO problem_draft_subtask_dependencies(problem_id,subtask_id,depends_on)VALUES($1,$2,$3);

-- name: InsertDraftTest :exec
INSERT INTO problem_draft_tests(problem_id,id,ordinal,subtask_id,sample,points,input_mode,input_object_id,generator_id,generator_args,seed,answer_mode,answer_object_id,solution_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14);

-- name: InsertDraftAsset :exec
INSERT INTO problem_draft_assets(problem_id,path,object_id)VALUES($1,$2,$3);

-- name: RefreshObjectReferences :exec
UPDATE stored_objects SET unreferenced_at=CASE WHEN stored_object_has_references(id) THEN NULL ELSE COALESCE(unreferenced_at,clock_timestamp()) END WHERE id=$1;
