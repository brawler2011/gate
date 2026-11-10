import {type ReactNode} from "react";
import {useLocation} from "react-router-dom";

import {AdaptiveTabs, type AdaptiveTabItem} from "@/components/shared/AdaptiveTabs";

const GENERAL_TAB = "general";
const TABS: Array<{key: string; label: string}> = [
  {key: GENERAL_TAB, label: "Общее"},
  {key: "statement", label: "Условие"},
  {key: "tests", label: "Тесты"},
  {key: "solutions", label: "Решения"},
  {key: "checkers", label: "Чекер"},
  {key: "validators", label: "Валидатор"},
  {key: "generators", label: "Генератор"},
  {key: "interactors", label: "Интерактор"},
  {key: "lib", label: "Библиотека"},
  {key: "packages", label: "Пакеты"},
  {key: "import", label: "Импорт"},
  {key: "access", label: "Доступ"},
];

export type ProblemHeaderNavProps = {
  slug: string;
  problemId: string;
};

export const ProblemHeaderNav = ({
  slug,
  problemId,
}: ProblemHeaderNavProps): ReactNode => {
  const {pathname} = useLocation();

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

  const items: AdaptiveTabItem[] = TABS.map((tab) => {
    const path =
      tab.key === GENERAL_TAB
        ? `/${slug}/problems/${problemId}`
        : `/${slug}/problems/${problemId}/${tab.key}`;
    return {
      key: tab.key,
      label: tab.label,
      href: path,
      active: tab.key === activeTab,
    };
  });

  return <AdaptiveTabs items={items} />;
};
