import {Alert, Container} from "@mantine/core";

import {Problem} from "./Problem";

import type {GetProblemResponseModel} from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

export const PublishedStatement = ({problem, statement}: GetProblemResponseModel): ReactNode => (
  <Container size="lg" py="md">
    {statement ? (
      <Problem
        problem={{...problem, ...statement, permissions: undefined}}
        problemId={problem.id}
        letter=""
        assetUrl={(path) => {
          const hash = statement.assets[path];
          return hash && `/api/problems/${problem.id}/assets/${hash}`;
        }}
        pdfUrl={`/api/problems/${problem.id}/statement.pdf`}
        filesUrl={statement.files ? `/api/problems/${problem.id}/files.zip` : undefined}
      />
    ) : (
      <Alert color="gray">Условие ещё не опубликовано.</Alert>
    )}
  </Container>
);
