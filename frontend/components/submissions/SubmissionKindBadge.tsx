import {Badge} from "@mantine/core";

import type {SubmissionKind} from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

const labels: Partial<Record<SubmissionKind, [string, string]>> = {
  upsolving: ["Дорешивание", "gray"],
  virtual: ["Виртуально", "violet"],
};

export const SubmissionKindBadge = ({kind}: {kind: SubmissionKind}): ReactNode => {
  const label = labels[kind];
  return label ? <Badge size="xs" variant="light" color={label[1]} tt="none">{label[0]}</Badge> : null;
};
