"use client";

import { WorkshopEditor } from "@/components/workshop";
import { Suspense } from "react";
import { useParams } from "next/navigation";

type ProblemPageProps = {
  activeTab: string;
};

export default function ProblemPage({ activeTab }: ProblemPageProps) {
  const params = useParams();
  const problem_id = params.problem_id as string;

  return (
    <Suspense>
      <WorkshopEditor problemId={problem_id} activeTab={activeTab} />
    </Suspense>
  );
}
