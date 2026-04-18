import { DefaultLayout } from "@/components/shared";
import { ErrorDisplay } from "@/components/shared/ErrorDisplay";
import { WorkshopEditor, WorkshopNotInitialized } from "@/components/workshop";
import { getProblem, getWorkshopProblemLimits } from "@/lib/actions";
import type { HeaderSecondaryNavItem } from "@/lib/contest-header-nav";
import { Metadata } from "next";
import { Suspense } from "react";

type SearchParams = Promise<{
  tab?: string;
  file?: string;
  [key: string]: string | string[] | undefined;
}>;

type Props = {
  params: Promise<{ problem_id: string }>;
  searchParams: SearchParams;
};

const GENERAL_TAB = "general";

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
  searchParams: Awaited<SearchParams>,
): string {
  const params = new URLSearchParams();

  for (const [key, value] of Object.entries(searchParams)) {
    if (key === "tab" || key === "file") {
      continue;
    }

    if (typeof value === "string") {
      params.set(key, value);
      continue;
    }

    if (Array.isArray(value)) {
      value.forEach((item) => {
        params.append(key, item);
      });
    }
  }

  params.set("tab", tab);

  return `/problems/${problemId}?${params.toString()}`;
}

function buildProblemHeaderNav(
  problemId: string,
  activeTab: string,
  searchParams: Awaited<SearchParams>,
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

export const generateMetadata = async (props: Props): Promise<Metadata> => {
  const { problem_id } = await props.params;

  const [error, response] = await getProblem(problem_id);
  if (error || !response) {
    return { title: "Редактор файлов" };
  }

  return { title: `Файлы — ${response.problem.title}` };
};

const Page = async (props: Props) => {
  const { problem_id } = await props.params;
  const resolvedSearchParams = await props.searchParams;
  const activeTab = resolvedSearchParams.tab ?? GENERAL_TAB;

  const [problemError] = await getProblem(problem_id);
  if (problemError) return <ErrorDisplay error={problemError} />;

  const [limitsError] = await getWorkshopProblemLimits(problem_id);
  const shouldRenderEditor = !limitsError;
  const problemHeaderNav = shouldRenderEditor
    ? buildProblemHeaderNav(problem_id, activeTab, resolvedSearchParams)
    : undefined;

  return (
    <DefaultLayout
      headerSecondaryNavItems={problemHeaderNav}
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
      {limitsError?.status === 404 ? (
        <WorkshopNotInitialized problemId={problem_id} />
      ) : limitsError ? (
        <ErrorDisplay error={limitsError} />
      ) : (
        <Suspense>
          <WorkshopEditor problemId={problem_id} />
        </Suspense>
      )}
    </DefaultLayout>
  );
};

export default Page;
