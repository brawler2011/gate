"use client";

import {Alert, Anchor, Group, Stack, Title} from "@mantine/core";
import {IconEyeOff, IconFileTypePdf} from "@tabler/icons-react";
import {type ReactNode} from "react";

import {ProblemCopyableBlock} from "./ProblemCopyableBlock";
import {ProblemHeader} from "./ProblemHeader";
import {ProblemLimits} from "./ProblemLimits";
import {ProblemSamples} from "./ProblemSamples";
import {type AssetUrl, StatementHtml} from "./StatementHtml";

import "./Problem.css";

type ProblemModelProps = {
  permissions?: {readonly edit_problem?: boolean};
  id?: string;
  problem_id?: string;
  organization_login?: string;
  title: string;
  time_limit: number;
  memory_limit: number;
  input_file?: string;
  output_file?: string;
  legend: string;
  input_format: string;
  output_format: string;
  notes: string;
  interaction?: string;
  scoring: string;
  tutorial?: string;
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
  assetUrl?: AssetUrl;
  pdfUrl?: string;
};

const Section = ({title, html, assetUrl}: {title: string; html?: string; assetUrl?: AssetUrl}): ReactNode =>
  html ? (
    <Stack gap="xs">
      <Title order={3}>{title}</Title>
      <StatementHtml html={html} assetUrl={assetUrl} />
    </Stack>
  ) : null;

const ProblemComponent = ({
  problem,
  letter = "A",
  problemId,
  isManager,
  orgLogin,
  hideStatements,
  assetUrl,
  pdfUrl,
}: ProblemProps): ReactNode => {
  const activeProblemId = problemId || problem.id || problem.problem_id;
  const org = orgLogin || problem.organization_login;

  return (
    <Stack className="container" gap="md">
      <ProblemHeader
        title={problem.title}
        letter={letter}
        timeLimit={problem.time_limit}
        memoryLimit={problem.memory_limit}
        inputFile={problem.input_file}
        outputFile={problem.output_file}
        isManager={Boolean(problem.permissions?.edit_problem)}
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
          {pdfUrl && (
            <Group justify="flex-end">
              <Anchor href={pdfUrl} target="_blank" rel="noreferrer" size="sm">
                <Group gap={4}>
                  <IconFileTypePdf size={16} />
                  PDF
                </Group>
              </Anchor>
            </Group>
          )}
          {problem.legend && <StatementHtml html={problem.legend} assetUrl={assetUrl} />}
          <Section title="Входные данные" html={problem.input_format} assetUrl={assetUrl} />
          <Section title="Выходные данные" html={problem.output_format} assetUrl={assetUrl} />
          <Section title="Протокол взаимодействия" html={problem.interaction} assetUrl={assetUrl} />
          <Section title="Система оценки" html={problem.scoring} assetUrl={assetUrl} />
          {problem.samples && problem.samples.length > 0 && (
            <ProblemSamples samples={problem.samples} />
          )}
          <Section title="Примечание" html={problem.notes} assetUrl={assetUrl} />
          <Section title="Разбор" html={problem.tutorial} assetUrl={assetUrl} />
        </>
      )}
    </Stack>
  );
};

export const Problem = Object.assign(ProblemComponent, {
  Header: ProblemHeader,
  Limits: ProblemLimits,
  Samples: ProblemSamples,
  CopyableBlock: ProblemCopyableBlock,
});
