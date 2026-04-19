import { HeaderWithSession } from "@/components/shared/HeaderWithSession";
import type { HeaderSecondaryNavItem } from "@/lib/contest-header-nav";
import type { AppShellProps } from "@mantine/core";
import { DefaultLayoutClient } from "./Layout";

type DefaultLayoutProps = {
  children: React.ReactNode;
  headerSecondaryNavItems?: HeaderSecondaryNavItem[];
  headerOrganizationId?: string;
  headerConfig?: AppShellProps["header"];
  footerConfig?: AppShellProps["footer"];
  asideConfig?: AppShellProps["aside"];
  navbarConfig?: AppShellProps["navbar"];
  stylesConfig?: AppShellProps["styles"];
  paddingConfig?: AppShellProps["padding"];
};

export async function DefaultLayout({
  children,
  headerSecondaryNavItems,
  headerOrganizationId,
  ...props
}: DefaultLayoutProps) {
  return (
    <DefaultLayoutClient
      {...props}
      header={
        <HeaderWithSession
          secondaryNavItems={headerSecondaryNavItems}
          organizationId={headerOrganizationId}
        />
      }
    >
      {children}
    </DefaultLayoutClient>
  );
}
