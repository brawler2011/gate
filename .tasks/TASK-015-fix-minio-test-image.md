---
id: TASK-015
title: "Fix MinIO test image registry for integration tests"
status: done
type: fix
description: "Switch MinIO container image registry from Docker Hub to Quay.io in backend integration tests."
priority: high
created_at: 2026-09-23
tags:
  - backend
  - ci
  - tests
  - backward compatible
---

# TASK-015: Fix MinIO test image registry for integration tests

## Context
In CI/CD backend integration tests, `TestS3ClientOperations` failed because testcontainers attempted to pull `minio/minio:latest` from Docker Hub, which returned `pull access denied: requested access to the resource is denied`. MinIO deprecated and removed their public images on Docker Hub in favor of `quay.io/minio/minio`. Switching the test container image to `quay.io/minio/minio:latest` resolves image pulling failures in CI.

## Acceptance Criteria
- [x] MinIO test container image updated to `quay.io/minio/minio:latest` in `backend/tests/integration/problem_package_and_storage_test.go`
- [x] `task tasks:validate` passes
- [x] Pre-commit checks pass

## Implementation Notes
Surgical change updating only the image reference for testcontainers. No functional or configuration changes to the S3 client or test logic.
