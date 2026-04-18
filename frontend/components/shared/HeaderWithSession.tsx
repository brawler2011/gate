import { getCurrentUser } from "@/lib/auth";
import type { HeaderSecondaryNavItem } from "@/lib/contest-header-nav";
import { Header } from "./Header";

type HeaderWithSessionProps = {
  secondaryNavItems?: HeaderSecondaryNavItem[];
};

export async function HeaderWithSession({
  secondaryNavItems,
}: HeaderWithSessionProps = {}) {
  const user = await getCurrentUser();

  return <Header user={user} secondaryNavItems={secondaryNavItems} />;
}
