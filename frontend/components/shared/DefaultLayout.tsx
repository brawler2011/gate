import {Header} from "@/components/shared/Header";

import {DefaultLayoutClient} from "./Layout";

import type {AdaptiveTabItem} from "@/components/shared/AdaptiveTabs";
import type {
  HeaderContest,
  HeaderOrganization,
  HeaderProblem,
} from "@/components/shared/Header";
import type {AppShellProps} from "@mantine/core";
import type {ReactNode} from "react";

export type DefaultLayoutProps = {
  children: ReactNode;
  headerSecondaryNav?: ReactNode;
  headerSecondaryNavItems?: AdaptiveTabItem[];
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
  headerSecondaryNav,
  headerSecondaryNavItems,
  headerOrganizationId: _headerOrganizationId,
  headerOrganization,
  headerContest,
  headerProblem,
  ...props
}: DefaultLayoutProps): ReactNode => {
  return (
    <DefaultLayoutClient
      {...props}
      header={
        <Header
          secondaryNav={headerSecondaryNav}
          secondaryNavItems={headerSecondaryNavItems}
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
