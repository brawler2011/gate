"use client";

import {usePathname, useSearchParams} from "next/navigation";
import {type ReactNode} from "react";

import {AdaptiveTabs, type AdaptiveTabItem} from "@/components/shared/AdaptiveTabs";

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

export type ProblemHeaderNavProps = {
  slug: string;
  problemId: string;
};

export const ProblemHeaderNav = ({
  slug,
  problemId,
}: ProblemHeaderNavProps): ReactNode => {
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const getActiveTabKey = (): string => {
    if (!pathname) {
      return GENERAL_TAB;
    }
    const basePath = `/${slug}/problems/${problemId}`;
    if (pathname === basePath || pathname === `${basePath}/`) {
      return GENERAL_TAB;
    }
    const sub = pathname.replace(`${basePath}/`, "").split("/")[0];
    return sub || GENERAL_TAB;
  };

  const activeTab = getActiveTabKey();

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

  const params = new URLSearchParams();
  searchParams.forEach((value, key) => {
    if (key !== "tab" && key !== "file") {
      params.append(key, value);
    }
  });
  const queryString = params.toString();

  const items: AdaptiveTabItem[] = tabs.map((tab) => {
    const path =
      tab.key === GENERAL_TAB
        ? `/${slug}/problems/${problemId}`
        : `/${slug}/problems/${problemId}/${tab.key}`;
    return {
      key: tab.key,
      label: tab.label,
      href: queryString ? `${path}?${queryString}` : path,
      active: tab.key === activeTab,
    };
  });

  return <AdaptiveTabs items={items} />;
};
