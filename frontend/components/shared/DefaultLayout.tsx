import { HeaderWithSession } from '@/components/shared/HeaderWithSession';
import { DefaultLayoutClient } from "./Layout";
import type { AppShellProps } from "@mantine/core";

type DefaultLayoutProps = {
  children: React.ReactNode;
  headerConfig?: AppShellProps["header"];
  footerConfig?: AppShellProps["footer"];
  asideConfig?: AppShellProps["aside"];
  navbarConfig?: AppShellProps["navbar"];
  stylesConfig?: AppShellProps["styles"];
  paddingConfig?: AppShellProps["padding"];
};

export async function DefaultLayout({ children, ...props }: DefaultLayoutProps) {
  return (
    <DefaultLayoutClient {...props} header={<HeaderWithSession />}>
      {children}
    </DefaultLayoutClient>
  );
}

