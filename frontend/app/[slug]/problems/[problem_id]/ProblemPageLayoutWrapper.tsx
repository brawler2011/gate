import {DefaultLayout} from "@/components/shared";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api} from "@/lib/api";

import ProblemPage from "./ProblemPage";

import type {AdaptiveTabItem} from "@/components/shared/AdaptiveTabs";
import type {Metadata} from "next";

const GENERAL_TAB = "general";
const WORKSHOP_FOLDER_TABS = [
  "checkers",
  "generators",
  "interactors",
  "lib",
  "media",
  "solutions",
  "tests",
  "validators",
] as const;

const TAB_LABELS: Record<string, string> = {
  checkers: "Чекер",
  generators: "Генератор",
  interactors: "Интерактор",
  lib: "Библиотека",
  media: "Медиа",
  solutions: "Решения",
  tests: "Тесты",
  validators: "Валидатор",
};

const buildProblemTabHref = (
  slug: string,
  problemId: string,
  tab: string,
  searchParams: Record<string, string | string[] | undefined>,
): string => {
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

  const path = tab === GENERAL_TAB ? `/${slug}/problems/${problemId}` : `/${slug}/problems/${problemId}/${tab}`;
  const queryString = params.toString();
  return queryString ? `${path}?${queryString}` : path;
};

const buildProblemHeaderNav = (
  slug: string,
  problemId: string,
  activeTab: string,
  searchParams: Record<string, string | string[] | undefined>,
): AdaptiveTabItem[] => {
  const tabs: Array<{ key: string; label: string }> = [
    {key: GENERAL_TAB, label: "Общее"},
    {key: "statement", label: "Условие"},
    {key: "access", label: "Доступ"},
    {key: "packages", label: "Пакеты"},
    {key: "import", label: "Импорт"},
    ...WORKSHOP_FOLDER_TABS.map((tab) => ({
      key: tab,
      label: TAB_LABELS[tab],
    })),
  ];

  return tabs.map((tab) => ({
    key: tab.key,
    label: tab.label,
    href: buildProblemTabHref(slug, problemId, tab.key, searchParams),
    active: tab.key === activeTab,
  }));
};

type Props = {
  activeTab: string;
  params: Promise<{ slug: string; problem_id: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
};

export const generateMetadata = async (problemId: string): Promise<Metadata> => {
  const [error, response] = await api.getProblem({id: problemId});
  if (error || !response) {
    return {title: "Редактор файлов"};
  }
  return {title: `Файлы — ${response.problem.title}`};
};

const ProblemPageLayoutWrapper = async ({
  activeTab,
  params,
  searchParams,
}: Props): Promise<JSX.Element> => {
  const {slug, problem_id} = await params;
  const resolvedSearchParams = await searchParams;

  const [
    [problemError, problemResponse],
    [, orgResponse],
    [limitsError],
  ] = await Promise.all([
    api.getProblem({id: problem_id}),
    api.getOrganization({login: slug}),
    api.getProblemLimits({problemId: problem_id}),
  ]);

  if (problemError) {
    return (
      <DefaultLayout>
        <ErrorDisplay error={problemError} />
      </DefaultLayout>
    );
  }

  const shouldRenderEditor = !limitsError;
  const problemHeaderNav = shouldRenderEditor
    ? buildProblemHeaderNav(slug, problem_id, activeTab, resolvedSearchParams)
    : undefined;

  const org = orgResponse?.organization;

  return (
    <DefaultLayout
      headerSecondaryNavItems={problemHeaderNav}
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
      {limitsError ? (
        <ErrorDisplay error={limitsError} />
      ) : (
        <ProblemPage activeTab={activeTab} />
      )}
    </DefaultLayout>
  );
};

export default ProblemPageLayoutWrapper;
