"use client";

import { Stack } from "@mantine/core";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback } from "react";
import { WorkshopCheckersTab } from "./WorkshopCheckersTab";
import { WorkshopGeneralTab } from "./WorkshopGeneralTab";
import { WorkshopGeneratorsTab } from "./WorkshopGeneratorsTab";
import {
  GENERAL_TAB,
  IMPORT_TAB,
  PACKAGES_TAB,
  STATEMENT_TAB,
  WorkshopHotbar,
} from "./WorkshopHotbar";
import { WorkshopImportTab } from "./WorkshopImportTab";
import { WorkshopInteractorsTab } from "./WorkshopInteractorsTab";
import { WorkshopMediaTab } from "./WorkshopMediaTab";
import { WorkshopPackagesTab } from "./WorkshopPackagesTab";
import { WorkshopSolutionsTab } from "./WorkshopSolutionsTab";
import { WorkshopStatementTab } from "./WorkshopStatementTab";
import { WorkshopTestsTab } from "./WorkshopTestsTab";
import { WorkshopValidatorsTab } from "./WorkshopValidatorsTab";

type Props = {
  problemId: string;
};

const WORKSHOP_FOLDERS = [
  "checkers",
  "generators",
  "interactors",
  "media",
  "solutions",
  "tests",
  "validators",
];

export function WorkshopEditor({ problemId }: Props) {
  const router = useRouter();
  const searchParams = useSearchParams();

  const activeTab = searchParams.get("tab") ?? GENERAL_TAB;
  const selectedFile = searchParams.get("file");

  const setTab = useCallback(
    (tab: string) => {
      const params = new URLSearchParams(searchParams.toString());
      params.set("tab", tab);
      params.delete("file");
      router.push(`?${params.toString()}`, { scroll: false });
    },
    [router, searchParams],
  );

  const setFile = useCallback(
    (filePath: string) => {
      const params = new URLSearchParams(searchParams.toString());
      params.set("file", filePath);
      router.push(`?${params.toString()}`, { scroll: false });
    },
    [router, searchParams],
  );

  const handleFileCreated = useCallback(
    (filePath: string) => {
      const params = new URLSearchParams(searchParams.toString());
      params.set("file", filePath);
      router.push(`?${params.toString()}`, { scroll: false });
    },
    [router, searchParams],
  );

  const renderFolderTab = () => {
    const folderTabProps = {
      problemId,
      selectedFile,
      onFileSelect: setFile,
      onFileCreated: handleFileCreated,
    };

    switch (activeTab) {
      case "checkers":
        return <WorkshopCheckersTab {...folderTabProps} />;
      case "generators":
        return <WorkshopGeneratorsTab {...folderTabProps} />;
      case "interactors":
        return <WorkshopInteractorsTab {...folderTabProps} />;
      case "media":
        return <WorkshopMediaTab {...folderTabProps} />;
      case "solutions":
        return <WorkshopSolutionsTab {...folderTabProps} />;
      case "tests":
        return <WorkshopTestsTab {...folderTabProps} />;
      case "validators":
        return <WorkshopValidatorsTab {...folderTabProps} />;
      default:
        return null;
    }
  };

  return (
    <Stack gap={0} style={{ height: "calc(100vh - 70px)" }}>
      <WorkshopHotbar
        folders={WORKSHOP_FOLDERS}
        activeTab={activeTab}
        onTabChange={setTab}
      />

      {/* Tab content */}
      <Stack gap={0} style={{ flex: 1, overflow: "hidden", display: "flex" }}>
        {activeTab === GENERAL_TAB ? (
          <WorkshopGeneralTab problemId={problemId} />
        ) : activeTab === STATEMENT_TAB ? (
          <WorkshopStatementTab problemId={problemId} />
        ) : activeTab === PACKAGES_TAB ? (
          <WorkshopPackagesTab problemId={problemId} />
        ) : activeTab === IMPORT_TAB ? (
          <WorkshopImportTab problemId={problemId} />
        ) : (
          renderFolderTab()
        )}
      </Stack>
    </Stack>
  );
}
