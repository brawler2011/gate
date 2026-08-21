import {DefaultLayout} from "@/components/shared";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {ProblemHeaderNav} from "@/components/workshop";
import {api} from "@/lib/api";

import type {Metadata} from "next";
import type {ReactNode} from "react";

type Props = {
  children: ReactNode;
  params: Promise<{ slug: string; problem_id: string }>;
};

export const generateMetadata = async ({
  params,
}: {
  params: Promise<{ slug: string; problem_id: string }>;
}): Promise<Metadata> => {
  const {problem_id} = await params;
  const [error, response] = await api.getProblem({id: problem_id});
  if (error || !response) {
    return {title: "Редактор файлов"};
  }
  return {title: `Файлы — ${response.problem.title}`};
};

const ProblemLayout = async ({children, params}: Props): Promise<ReactNode> => {
  const {slug, problem_id} = await params;

  const [
    [problemError, problemResponse],
    [, orgResponse],
  ] = await Promise.all([
    api.getProblem({id: problem_id}),
    api.getOrganization({login: slug}),
  ]);

  if (problemError) {
    return (
      <DefaultLayout>
        <ErrorDisplay error={problemError} />
      </DefaultLayout>
    );
  }

  const org = orgResponse?.organization;

  return (
    <DefaultLayout
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
      {children}
    </DefaultLayout>
  );
};

export default ProblemLayout;
