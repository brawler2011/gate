"use client";

import {Container} from "@mantine/core";

import {ProblemMembersManagement} from "@/components/problems/ProblemMembersManagement";

import type {ReactNode} from "react";

type Props = {
  problemId: string;
};

export const WorkshopAccessTab = ({problemId}: Props): ReactNode => (
  <Container size="lg" py="lg">
    <ProblemMembersManagement problemId={problemId} />
  </Container>
);
