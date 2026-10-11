---
id: TASK-017
title: "Use SeaweedFS for S3 storage integration tests"
status: done
type: fix
description: "Replace the unavailable MinIO test image with the existing SeaweedFS fixture."
priority: high
created_at: 2026-10-11
tags:
  - backend
  - tests
  - backward compatible
---

# TASK-017: Use SeaweedFS for S3 storage integration tests

## Context
TestS3ClientOperations pulls quay.io/minio/minio:latest, which fails on clean
GitHub runners with unauthorized. Cached local MinIO images hide this failure.
Deployment and judge E2E already use chrislusf/seaweedfs:3.77_full. Reuse the
existing newSeaweedBackedS3Storage fixture so S3 operations are tested against
the actual storage implementation, with no MinIO dependency in executable tests.
Keep this separate from TASK-016 and preserve all existing S3 assertions.

## Acceptance Criteria
- [x] TestS3ClientOperations uses the existing SeaweedFS fixture and no executable test references MinIO.
- [x] Bucket creation, upload, download/content, listing, presigned HTTP GET and deletion checks remain intact and pass.
- [x] The complete backend integration suite, task tasks:validate and task precommit pass.

## Implementation Notes
No production, deployment, frontend, storage client or CI workflow changes.
Open a separate PR with independent review and inspect GitHub CI; do not merge.

Verification: the targeted TestS3ClientOperations test and full backend integration
suite passed against SeaweedFS; task precommit passed, including task validation.
