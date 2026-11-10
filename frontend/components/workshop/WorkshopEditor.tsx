import {Stack} from "@mantine/core";

import {WorkshopAccessTab} from "./WorkshopAccessTab";
import {WorkshopComponentsTab} from "./WorkshopComponentsTab";
import classes from "./WorkshopEditor.module.css";
import {WorkshopGeneralTab} from "./WorkshopGeneralTab";
import {WorkshopImportTab} from "./WorkshopImportTab";
import {WorkshopLibTab} from "./WorkshopLibTab";
import {WorkshopPackagesTab} from "./WorkshopPackagesTab";
import {WorkshopStatementTab} from "./WorkshopStatementTab";
import {WorkshopTestsTab} from "./WorkshopTestsTab";

import type {ComponentRole} from "./draft/draftOps";
import type {ReactNode} from "react";

type Props = {
  problemId: string;
  activeTab: string;
};

/** Route segment → component role edited on that tab. */
const COMPONENT_TABS: Record<string, ComponentRole> = {
  checkers: "checker",
  validators: "validator",
  interactors: "interactor",
  generators: "generator",
  solutions: "solution",
};

export const WorkshopEditor = ({problemId, activeTab}: Props): ReactNode => {
  const renderTabContent = () => {
    const role = COMPONENT_TABS[activeTab];
    if (role) {
      return <WorkshopComponentsTab problemId={problemId} role={role} />;
    }
    switch (activeTab) {
      case "statement":
        return <WorkshopStatementTab problemId={problemId} />;
      case "tests":
        return <WorkshopTestsTab problemId={problemId} />;
      case "lib":
        return <WorkshopLibTab problemId={problemId} />;
      case "access":
        return <WorkshopAccessTab problemId={problemId} />;
      case "packages":
        return <WorkshopPackagesTab problemId={problemId} />;
      case "import":
        return <WorkshopImportTab problemId={problemId} />;
      default:
        return <WorkshopGeneralTab problemId={problemId} />;
    }
  };

  return (
    <Stack gap={0} className={classes.root}>
      <Stack gap={0} className={classes.content}>
        {renderTabContent()}
      </Stack>
    </Stack>
  );
};
