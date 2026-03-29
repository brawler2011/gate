"use client";

import { Box } from "@mantine/core";
import classes from "./WorkshopHotbar.module.css";

export const GENERAL_TAB = "general";
export const STATEMENT_TAB = "statement";
export const PACKAGES_TAB = "packages";
export const IMPORT_TAB = "import";

type Props = {
  folders: string[];
  activeTab: string;
  onTabChange: (tab: string) => void;
};

const TAB_LABELS: Record<string, string> = {
  checkers: "Чекеры",
  generators: "Генераторы",
  interactors: "Интеракторы",
  media: "Медиа",
  packages: "Пакеты",
  solutions: "Решения",
  tests: "Тесты",
  validators: "Валидаторы",
};

function getTabLabel(folder: string): string {
  return TAB_LABELS[folder] ?? folder.charAt(0).toUpperCase() + folder.slice(1);
}

export function WorkshopHotbar({ folders, activeTab, onTabChange }: Props) {
  const allTabs = [GENERAL_TAB, STATEMENT_TAB, PACKAGES_TAB, IMPORT_TAB, ...folders.sort()];

  return (
    <Box className={classes.hotbar}>
      <div className={classes.inner}>
        {allTabs.map((tab) => (
          <button
            key={tab}
            className={`${classes.tab} ${activeTab === tab ? classes.tabActive : ""}`}
            onClick={() => onTabChange(tab)}
            type="button"
          >
            {tab === GENERAL_TAB
              ? "Общее"
              : tab === STATEMENT_TAB
                ? "Условие"
                : tab === PACKAGES_TAB
                  ? "Пакеты"
                  : tab === IMPORT_TAB
                    ? "Импорт"
                    : getTabLabel(tab)}
          </button>
        ))}
      </div>
    </Box>
  );
}
