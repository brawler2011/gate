import {Badge} from "@mantine/core";

import type {SubmissionKind} from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

export const SubmissionKindBadge = ({kind}: {kind: SubmissionKind}): ReactNode =>
  kind === "upsolving" ? <Badge size="xs" variant="light" color="gray" tt="none">Дорешивание</Badge> : null;
