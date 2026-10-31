import {Suspense, type ReactNode} from "react";
import {useParams} from "react-router-dom";

import {WorkshopEditor} from "@/components/workshop/WorkshopEditor";
import {api} from "@/lib/api";

type ProblemPageProps = {
  activeTab: string;
  title: string;
};

const ProblemPage = ({activeTab, title}: ProblemPageProps): ReactNode => {
  const params = useParams();
  const problem_id = params.problem_id as string;
  const problem = api.useQuery("get", "/problems/{id}", {params: {path: {id: problem_id}}});

  return (
    <Suspense>
      <title>{activeTab === "general" ? problem.data?.problem.title ?? "Задача" : title}</title>
      <WorkshopEditor problemId={problem_id} activeTab={activeTab} />
    </Suspense>
  );
};

export default ProblemPage;
