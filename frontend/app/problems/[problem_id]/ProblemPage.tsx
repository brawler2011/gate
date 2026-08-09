"use client";

import {useParams} from "next/navigation";
import {Suspense} from "react";

import {WorkshopEditor} from "@/components/workshop";

type ProblemPageProps = {
  activeTab: string;
};

const ProblemPage = ({activeTab}: ProblemPageProps) => {
  const params = useParams();
  const problem_id = params.problem_id as string;

  return (
    <Suspense>
      <WorkshopEditor problemId={problem_id} activeTab={activeTab} />
    </Suspense>
  );
};

export default ProblemPage;
