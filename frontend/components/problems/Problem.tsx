"use client";

import {Alert, Stack, Title} from "@mantine/core";
import {IconEyeOff} from "@tabler/icons-react";
import katex from "katex";
import {useEffect, useRef, type ReactNode} from "react";

import {ProblemCopyableBlock} from "./ProblemCopyableBlock";
import {ProblemHeader} from "./ProblemHeader";
import {ProblemLimits} from "./ProblemLimits";
import {ProblemSamples} from "./ProblemSamples";
import {type AssetUrl, ProblemStatement} from "./ProblemStatement";

import "./Problem.css";

type ProblemModelProps = {
  capabilities?: readonly string[];
  id?: string;
  problem_id?: string;
  organization_login?: string;
  title: string;
  time_limit: number;
  memory_limit: number;
  legend: string;
  input_format: string;
  output_format: string;
  notes: string;
  scoring: string;
  created_at: string;
  updated_at: string;
  samples?: Array<{
    input: string;
    output: string;
  }>;
};

export type ProblemProps = {
  problem: ProblemModelProps;
  orgLogin?: string;
  problemId?: string;
  letter?: string;
  isManager?: boolean;
  hideStatements?: boolean;
  /** Resolves statement image paths to URLs. */
  assetUrl?: AssetUrl;
};

const ProblemComponent = ({
  problem,
  letter = "A",
  problemId,
  isManager,
  orgLogin,
  hideStatements,
  assetUrl,
}: ProblemProps): ReactNode => {
  const activeProblemId = problemId || problem.id || problem.problem_id;
  const org = orgLogin || problem.organization_login;

  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const containerNode = ref.current;
    if (containerNode) {
      const mathElements = containerNode.querySelectorAll(".math");
      mathElements.forEach((element) => {
        if (
          element instanceof HTMLElement &&
          !element.hasAttribute("data-rendered")
        ) {
          katex.render(element.textContent || "", element, {
            throwOnError: false,
            displayMode: element.classList.contains("display"),
          });

          element.setAttribute("data-rendered", "true");
        }
      });
    }

    return () => {
      if (containerNode) {
        containerNode.querySelectorAll(".math").forEach((element) => {
          element.removeAttribute("data-rendered");
        });
      }
    };
  }, [problem, letter]);

  return (
    <Stack className="container" ref={ref} gap="md">
      <ProblemHeader
        title={problem.title}
        letter={letter}
        timeLimit={problem.time_limit}
        memoryLimit={problem.memory_limit}
        isManager={Boolean(problem.capabilities?.includes("edit"))}
        problemId={activeProblemId}
        orgLogin={org}
      />

      {hideStatements && !isManager ? (
        <Alert
          icon={<IconEyeOff size={20} />}
          title="Условия задач скрыты"
          color="blue"
          variant="light"
          radius="md"
        >
          Условия задач скрыты организатором контеста.
          Пожалуйста, используйте печатные материалы для ознакомления с заданием.
        </Alert>
      ) : (
        <>
          {problem.legend && (
            <ProblemStatement value={problem.legend} assetUrl={assetUrl} />
          )}

          {problem.input_format && (
            <Stack gap="xs">
              <Title order={3}>Входные данные</Title>
              <ProblemStatement value={problem.input_format} assetUrl={assetUrl} />
            </Stack>
          )}

          {problem.output_format && (
            <Stack gap="xs">
              <Title order={3}>Выходные данные</Title>
              <ProblemStatement value={problem.output_format} assetUrl={assetUrl} />
            </Stack>
          )}

          {problem.samples && problem.samples.length > 0 && (
            <ProblemSamples samples={problem.samples} />
          )}

          {problem.notes && (
            <Stack gap="xs">
              <Title order={3}>Примечание</Title>
              <ProblemStatement value={problem.notes} assetUrl={assetUrl} />
            </Stack>
          )}
        </>
      )}
    </Stack>
  );
};

export const Problem = Object.assign(ProblemComponent, {
  Header: ProblemHeader,
  Limits: ProblemLimits,
  Statement: ProblemStatement,
  Samples: ProblemSamples,
  CopyableBlock: ProblemCopyableBlock,
});
