"use client";

import {Suspense, type ReactNode} from "react";

import {WorkshopEditor} from "@/components/workshop";
import {useParams} from "@/lib/navigation";

type ProblemPageProps = {
  activeTab: string;
};

const ProblemPage = ({activeTab}: ProblemPageProps): ReactNode => {
  const params = useParams();
  const problem_id = params.problem_id as string;

  return (
    <Suspense>
      <WorkshopEditor problemId={problem_id} activeTab={activeTab} />
    </Suspense>
  );
};

export default ProblemPage;
