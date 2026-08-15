import {HeaderWithSession} from "@/components/shared/HeaderWithSession";

import {DefaultLayoutClient} from "./Layout";

import type {
  HeaderContest,
  HeaderOrganization,
  HeaderProblem,
} from "@/components/shared/Header";
import type {HeaderSecondaryNavItem} from "@/lib/contest-header-nav";
import type {AppShellProps} from "@mantine/core";
import type {ReactNode} from "react";

type DefaultLayoutProps = {
  children: React.ReactNode;
  headerSecondaryNavItems?: HeaderSecondaryNavItem[];
  headerOrganizationId?: string;
  headerOrganization?: HeaderOrganization;
  headerContest?: HeaderContest;
  headerProblem?: HeaderProblem;
  headerConfig?: AppShellProps["header"];
  footerConfig?: AppShellProps["footer"];
  asideConfig?: AppShellProps["aside"];
  navbarConfig?: AppShellProps["navbar"];
  stylesConfig?: AppShellProps["styles"];
  paddingConfig?: AppShellProps["padding"];
};

export const DefaultLayout = ({
  children,
  headerSecondaryNavItems,
  headerOrganizationId,
  headerOrganization,
  headerContest,
  headerProblem,
  ...props
}: DefaultLayoutProps): ReactNode => {
  return (
    <DefaultLayoutClient
      {...props}
      header={
        <HeaderWithSession
          secondaryNavItems={headerSecondaryNavItems}
          organizationId={headerOrganizationId}
          organization={headerOrganization}
          contest={headerContest}
          problem={headerProblem}
        />
      }
    >
      {children}
    </DefaultLayoutClient>
  );
};
