"use client";

import { Stack } from "@mantine/core";
import { useRouter, useSearchParams, usePathname } from "next/navigation";
import { useCallback } from "react";
import { WorkshopCheckersTab } from "./WorkshopCheckersTab";
import classes from "./WorkshopEditor.module.css";
import { WorkshopGeneralTab } from "./WorkshopGeneralTab";
import { WorkshopGeneratorsTab } from "./WorkshopGeneratorsTab";
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
  activeTab: string;
};

const GENERAL_TAB = "general";
const STATEMENT_TAB = "statement";
const PACKAGES_TAB = "packages";
const IMPORT_TAB = "import";

export function WorkshopEditor({ problemId, activeTab }: Props) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const pathname = usePathname();

  const selectedFile = searchParams.get("file");

  const setFile = useCallback(
    (filePath: string) => {
      const params = new URLSearchParams(searchParams.toString());
      params.set("file", filePath);
      router.push(`${pathname}?${params.toString()}`, { scroll: false });
    },
    [router, searchParams, pathname],
  );

  const handleFileCreated = useCallback(
    (filePath: string) => {
      const params = new URLSearchParams(searchParams.toString());
      params.set("file", filePath);
      router.push(`${pathname}?${params.toString()}`, { scroll: false });
    },
    [router, searchParams, pathname],
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
    <Stack gap={0} className={classes.root}>
      {/* Tab content */}
      <Stack gap={0} className={classes.content}>
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
