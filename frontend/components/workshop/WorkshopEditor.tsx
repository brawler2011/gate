import {Stack} from "@mantine/core";
import {useCallback} from "react";
import {useNavigate, useSearchParams, useLocation} from "react-router-dom";

import {WorkshopAccessTab} from "./WorkshopAccessTab";
import {WorkshopCheckersTab} from "./WorkshopCheckersTab";
import classes from "./WorkshopEditor.module.css";
import {WorkshopGeneralTab} from "./WorkshopGeneralTab";
import {WorkshopGeneratorsTab} from "./WorkshopGeneratorsTab";
import {WorkshopImportTab} from "./WorkshopImportTab";
import {WorkshopInteractorsTab} from "./WorkshopInteractorsTab";
import {WorkshopLibTab} from "./WorkshopLibTab";
import {WorkshopMediaTab} from "./WorkshopMediaTab";
import {WorkshopPackagesTab} from "./WorkshopPackagesTab";
import {WorkshopSolutionsTab} from "./WorkshopSolutionsTab";
import {WorkshopStatementTab} from "./WorkshopStatementTab";
import {WorkshopTestsTab} from "./WorkshopTestsTab";
import {WorkshopValidatorsTab} from "./WorkshopValidatorsTab";

import type {ReactNode} from "react";

type Props = {
  problemId: string;
  activeTab: string;
};

const GENERAL_TAB = "general";
const STATEMENT_TAB = "statement";
const ACCESS_TAB = "access";
const PACKAGES_TAB = "packages";
const IMPORT_TAB = "import";

export const WorkshopEditor = ({problemId, activeTab}: Props): ReactNode => {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const {pathname} = useLocation();

  const selectedFile = searchParams.get("file");

  const setFile = useCallback(
    (filePath: string) => {
      const params = new URLSearchParams(searchParams.toString());
      params.set("file", filePath);
      navigate(`${pathname}?${params.toString()}`, {preventScrollReset: true});
    },
    [navigate, searchParams, pathname],
  );

  const handleFileCreated = useCallback(
    (filePath: string) => {
      const params = new URLSearchParams(searchParams.toString());
      params.set("file", filePath);
      navigate(`${pathname}?${params.toString()}`, {preventScrollReset: true});
    },
    [navigate, searchParams, pathname],
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
      case "lib":
        return <WorkshopLibTab {...folderTabProps} />;
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

  const renderTabContent = () => {
    if (activeTab === GENERAL_TAB) {
      return <WorkshopGeneralTab problemId={problemId} />;
    }
    if (activeTab === STATEMENT_TAB) {
      return <WorkshopStatementTab problemId={problemId} />;
    }
    if (activeTab === ACCESS_TAB) {
      return <WorkshopAccessTab problemId={problemId} />;
    }
    if (activeTab === PACKAGES_TAB) {
      return <WorkshopPackagesTab problemId={problemId} />;
    }
    if (activeTab === IMPORT_TAB) {
      return <WorkshopImportTab problemId={problemId} />;
    }
    return renderFolderTab();
  };

  return (
    <Stack gap={0} className={classes.root}>
      {/* Tab content */}
      <Stack gap={0} className={classes.content}>
        {renderTabContent()}
      </Stack>
    </Stack>
  );
};
