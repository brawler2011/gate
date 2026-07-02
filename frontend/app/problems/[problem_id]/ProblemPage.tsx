"use client";

import { DefaultLayout } from "@/components/shared";
import { ErrorDisplay } from "@/components/shared/ErrorDisplay";
import { WorkshopEditor } from "@/components/workshop";
import { getProblem, getWorkshopProblemLimits } from "@/lib/actions";
import type { HeaderSecondaryNavItem } from "@/lib/contest-header-nav";
import { Suspense, useEffect } from "react";
import useSWR from "swr";
import { useParams, useSearchParams } from "next/navigation";
import { Skeleton, Container } from "@mantine/core";

export const GENERAL_TAB = "general";

const WORKSHOP_FOLDER_TABS = [
  "checkers",
  "generators",
  "interactors",
  "media",
  "solutions",
  "tests",
  "validators",
] as const;

const TAB_LABELS: Record<string, string> = {
  checkers: "Чекеры",
  generators: "Генераторы",
  interactors: "Интеракторы",
  media: "Медиа",
  solutions: "Решения",
  tests: "Тесты",
  validators: "Валидаторы",
};

function buildProblemTabHref(
  problemId: string,
  tab: string,
  searchParams: URLSearchParams,
): string {
  const params = new URLSearchParams();

  searchParams.forEach((value, key) => {
    if (key !== "tab" && key !== "file") {
      params.append(key, value);
    }
  });

  const path = tab === GENERAL_TAB ? `/problems/${problemId}` : `/problems/${problemId}/${tab}`;
  const queryString = params.toString();
  return queryString ? `${path}?${queryString}` : path;
}

function buildProblemHeaderNav(
  problemId: string,
  activeTab: string,
  searchParams: URLSearchParams,
): HeaderSecondaryNavItem[] {
  const tabs: Array<{ key: string; label: string }> = [
    { key: GENERAL_TAB, label: "Общее" },
    { key: "statement", label: "Условие" },
    { key: "packages", label: "Пакеты" },
    { key: "import", label: "Импорт" },
    ...WORKSHOP_FOLDER_TABS.map((tab) => ({
      key: tab,
      label: TAB_LABELS[tab],
    })),
  ];

  return tabs.map((tab) => ({
    key: tab.key,
    label: tab.label,
    href: buildProblemTabHref(problemId, tab.key, searchParams),
    active: tab.key === activeTab,
  }));
}

type ProblemPageProps = {
  activeTab: string;
};

export default function ProblemPage({ activeTab }: ProblemPageProps) {
  const params = useParams();
  const searchParams = useSearchParams();
  const problem_id = params.problem_id as string;

  const { data: problemResponse, error: problemError, isLoading: isLoadingProblem } = useSWR(
    ["problem", problem_id],
    async () => {
      const [err, res] = await getProblem(problem_id);
      if (err) throw err;
      return res;
    }
  );

  const { data: limitsResponse, error: limitsError, isLoading: isLoadingLimits } = useSWR(
    ["problem-limits", problem_id],
    async () => {
      const [err, res] = await getWorkshopProblemLimits(problem_id);
      if (err) throw err;
      return res;
    }
  );

  useEffect(() => {
    if (problemResponse?.problem.title) {
      document.title = `Файлы — ${problemResponse.problem.title}`;
    } else {
      document.title = "Редактор файлов";
    }
  }, [problemResponse]);

  if (isLoadingProblem || isLoadingLimits) {
    return (
      <DefaultLayout
        stylesConfig={{
          header: { position: "static" },
          footer: { position: "static", bottom: "auto", width: "100%", zIndex: "auto" },
          main: { paddingTop: 0, paddingBottom: 0 },
        }}
      >
        <Container size="xl" py="xl">
          <Skeleton height={50} width={250} radius="sm" mb="md" />
          <Skeleton height={400} radius="sm" />
        </Container>
      </DefaultLayout>
    );
  }

  if (problemError) {
    return (
      <DefaultLayout>
        <ErrorDisplay error={problemError} />
      </DefaultLayout>
    );
  }

  const shouldRenderEditor = !limitsError;
  const urlSearchParams = new URLSearchParams(searchParams.toString());
  const problemHeaderNav = shouldRenderEditor
    ? buildProblemHeaderNav(problem_id, activeTab, urlSearchParams)
    : undefined;

  return (
    <DefaultLayout
      headerSecondaryNavItems={problemHeaderNav}
      headerOrganizationId={problemResponse?.problem.organization_id}
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
          paddingTop: 0,
          paddingBottom: 0,
        },
      }}
    >
      {limitsError ? (
        <ErrorDisplay error={limitsError} />
      ) : (
        <Suspense>
          <WorkshopEditor problemId={problem_id} activeTab={activeTab} />
        </Suspense>
      )}
    </DefaultLayout>
  );
}
