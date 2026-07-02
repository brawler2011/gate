import { DefaultLayout } from "@/components/shared";
import { ErrorDisplay } from "@/components/shared/ErrorDisplay";
import { getProblem, getWorkshopProblemLimits } from "@/lib/actions";
import type { HeaderSecondaryNavItem } from "@/lib/contest-header-nav";
import ProblemPage from "./ProblemPage";
import type { Metadata } from "next";

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
  searchParams: Record<string, string | string[] | undefined>,
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

  const path = tab === GENERAL_TAB ? `/problems/${problemId}` : `/problems/${problemId}/${tab}`;
  const queryString = params.toString();
  return queryString ? `${path}?${queryString}` : path;
}

function buildProblemHeaderNav(
  problemId: string,
  activeTab: string,
  searchParams: Record<string, string | string[] | undefined>,
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

type Props = {
  activeTab: string;
  params: Promise<{ problem_id: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
};

export async function generateMetadata(problemId: string): Promise<Metadata> {
  const [error, response] = await getProblem(problemId);
  if (error || !response) {
    return { title: "Редактор файлов" };
  }
  return { title: `Файлы — ${response.problem.title}` };
}

export default async function ProblemPageLayoutWrapper({
  activeTab,
  params,
  searchParams,
}: Props) {
  const { problem_id } = await params;
  const resolvedSearchParams = await searchParams;

  const [problemError, problemResponse] = await getProblem(problem_id);
  if (problemError) {
    return (
      <DefaultLayout>
        <ErrorDisplay error={problemError} />
      </DefaultLayout>
    );
  }

  const [limitsError] = await getWorkshopProblemLimits(problem_id);
  const shouldRenderEditor = !limitsError;
  const problemHeaderNav = shouldRenderEditor
    ? buildProblemHeaderNav(problem_id, activeTab, resolvedSearchParams)
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
        <ProblemPage activeTab={activeTab} />
      )}
    </DefaultLayout>
  );
}
