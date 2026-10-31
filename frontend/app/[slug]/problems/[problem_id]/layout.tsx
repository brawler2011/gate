import {Outlet, useParams} from "react-router-dom";

import {DefaultLayout} from "@/components/shared/DefaultLayout";
import {QueryState} from "@/components/shared/QueryState";
import {ProblemHeaderNav} from "@/components/workshop/ProblemHeaderNav";
import {useSession} from "@/contexts/SessionContext";
import {api} from "@/lib/api";

import type {ReactNode} from "react";

const ProblemLayout = (): ReactNode => {
  const {slug, problem_id} = useParams() as { slug: string; problem_id: string };
  const {user} = useSession();
  const problem = api.useQuery("get", "/problems/{id}", {params: {path: {id: problem_id}}});
  const organization = api.useQuery("get", "/organizations/{login}", {params: {path: {login: slug}}});
  if (!problem.data) {
    return (
      <DefaultLayout headerUser={user}>
        <QueryState error={problem.error} />
      </DefaultLayout>
    );
  }
  const problemResponse = problem.data;
  const org = organization.data?.organization;
  return (
    <DefaultLayout
      headerUser={user}
      headerSecondaryNav={<ProblemHeaderNav slug={slug} problemId={problem_id} />}
      headerOrganization={org ? {id: org.id, login: org.login, name: org.name} : undefined}
      headerProblem={
        problemResponse?.problem
          ? {
            id: problemResponse.problem.id,
            title: problemResponse.problem.title,
          }
          : undefined
      }
      stylesConfig={{
        header: {
          position: "static",
        },
        footer: {
          position: "static",
          bottom: "auto",
          width: "100%",
          zIndex: "auto",
        },
        main: {
          padding: 0,
          minHeight: "calc(100vh - 92px)",
        },
      }}
    >
      <Outlet />
    </DefaultLayout>
  );
};

export default ProblemLayout;
