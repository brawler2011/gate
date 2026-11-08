# Vacuum Schema-Order Example Check Reproducer

This directory documents an order-dependent false-positive behavior in Vacuum v0.33.0's `oas3-missing-example` rule (tracked in Issue #142).

## Bug Description

When analyzing schemas with whole-object `example:` blocks (such as multipart upload bodies or inline payloads), Vacuum's traversal and deduplication logic depends on the declaration order of schemas under `components.schemas`.

- In `vacuum_order_repro.yaml`, `First` appears before `UploadBody`. Running Vacuum produces **0** `oas3-missing-example` findings.
- Reversing only the order of `First` and `UploadBody` under `components.schemas` produces **1** `oas3-missing-example` finding for `UploadBody.file`, even though `UploadBody` defines a valid whole-object `example` containing `file`.

## Verification Command

```bash
vacuum lint tools/repro/vacuum_order_repro.yaml --ruleset vacuum.yaml --no-style --no-banner --no-update-check
```

In the main contract (`contracts/core/v1/openapi.yaml`), the 12 remaining `oas3-missing-example` warnings on inline payloads/multipart fields exhibit this exact behavior. As documented in Issue #142, moving schemas around or fabricating dummy examples merely to exploit Vacuum's traversal order is intentionally avoided.
