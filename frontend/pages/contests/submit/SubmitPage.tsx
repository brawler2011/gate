import {Container} from "@mantine/core";
import {useParams, Navigate} from "react-router-dom";

import {QueryState} from "@/components/shared/QueryState";
import {useContestData} from "@/hooks/useContestData";

import {SubmitSubmissionClient} from "./SubmitSubmissionClient";

import type {ReactNode} from "react";

const Page = (): ReactNode => {
  const params = useParams() as {slug: string; contestLogin: string};
  const {slug, contestLogin} = params;
  const routeQuery = useContestData(slug, contestLogin);
  if (!routeQuery.data) {
    return <QueryState error={routeQuery.error} />;
  }
  const {response, user} = routeQuery.data;
  if (!response.contest.permissions.submit) {
    return <Navigate to={`/${slug}/${contestLogin}`} replace />;
  }
  return (
    <Container size="lg" pb={{base: "md", sm: "lg", md: "xl"}}>
      <title>Отправить решение</title>
      <SubmitSubmissionClient contest={response.contest} problems={response.problems || []} user={user} />
    </Container>
  );
};

export default Page;
