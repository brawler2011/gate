"use client";

import {
  ActionIcon,
  Anchor,
  Avatar,
  Box,
  Burger,
  Button,
  Center,
  Divider,
  Drawer,
  Group,
  Image,
  Loader,
  Menu,
  ScrollArea,
  Skeleton,
  Stack,
  Title,
  useComputedColorScheme,
  useMantineColorScheme,
} from "@mantine/core";
import {useDisclosure} from "@mantine/hooks";
import {
  IconBuilding,
  IconLogout,
  IconMoon,
  IconSun,
  IconUser,
} from "@tabler/icons-react";
import cx from "clsx";
import NextImage from "next/image";
import Link from "next/link";
import {usePathname, useRouter} from "next/navigation";
import {useState, type ReactNode} from "react";

import {AdaptiveTabs, type AdaptiveTabItem} from "@/components/shared/AdaptiveTabs";
import {useSession} from "@/contexts/SessionContext";
import {api} from "@/lib/api";
import {APP_COLORS} from "@/lib/theme/colors";

import classes from "./Header.module.css";
import {LogoutLink} from "./LogoutLink";

import type {UserModel} from "@/contracts/core/v1";

export type HeaderOrganization = {
  id: string;
  login?: string;
  name: string;
};

export type HeaderContest = {
  id: string;
  login?: string;
  title: string;
};

export type HeaderProblem = {
  id: string;
  title: string;
};

const Profile = ({user: propUser}: { user?: UserModel | null }) => {
  const pathname = usePathname();
  const router = useRouter();
  const {user: sessionUser, setUser, isLoading} = useSession();
  const user = propUser !== undefined ? propUser : sessionUser;
  const [logoutLoading, setLogoutLoading] = useState(false);

  const getReturnUrl = () => {
    if (!pathname || pathname === "/" || pathname.startsWith("/auth")) {
      return null;
    }
    return pathname;
  };
  const returnUrl = getReturnUrl();
  const returnTo = returnUrl
    ? `?return_to=${encodeURIComponent(returnUrl)}`
    : "";

  const handleLogout = async () => {
    setLogoutLoading(true);
    try {
      await api.logout();
      setUser(null);
      router.push("/auth/login");
      router.refresh();
    } catch {
      setUser(null);
      router.push("/auth/login");
      router.refresh();
    } finally {
      setLogoutLoading(false);
    }
  };

  if (logoutLoading) {
    return (
      <Group justify="flex-end">
        <Center w={36} h={36}>
          <Loader size="xs" />
        </Center>
      </Group>
    );
  }

  if (isLoading && propUser === undefined && sessionUser === null) {
    return (
      <Group justify="flex-end">
        <Skeleton circle height={36} />
      </Group>
    );
  }

  if (user) {
    return (
      <Group justify="flex-end">
        <Menu
          shadow="md"
          width={200}
          position="bottom-end"
          transitionProps={{transition: "pop-top-right"}}
        >
          <Menu.Target>
            <Avatar
              color={APP_COLORS.users}
              size={36}
              style={{cursor: "pointer"}}
            >
              <IconUser size={20} />
            </Avatar>
          </Menu.Target>

          <Menu.Dropdown>
            <Menu.Item
              component={Link}
              href={`/@${user.username}`}
              leftSection={<IconUser size={16} />}
            >
              Профиль
            </Menu.Item>

            <Menu.Item
              component={Link}
              href="/orgs"
              leftSection={<IconBuilding size={16} />}
            >
              Организации
            </Menu.Item>

            <Menu.Divider />

            <Menu.Item
              color="red"
              onClick={handleLogout}
              leftSection={<IconLogout size={16} />}
            >
              Выйти
            </Menu.Item>
          </Menu.Dropdown>
        </Menu>
      </Group>
    );
  }

  return (
    <Group justify="flex-end">
      <Button
        component={Link}
        href={`/auth/login${returnTo}`}
        variant="filled"
        color={APP_COLORS.actions.primary}
        size="sm"
      >
        Войти
      </Button>
    </Group>
  );
};

export type HeaderProps = {
  user?: UserModel | null;
  secondaryNav?: ReactNode;
  secondaryNavItems?: AdaptiveTabItem[];
  organization?: HeaderOrganization;
  contest?: HeaderContest;
  problem?: HeaderProblem;
};

export const Header = ({
  user: propUser,
  secondaryNav,
  secondaryNavItems,
  organization,
  contest,
  problem,
}: HeaderProps): ReactNode => {
  const [drawerOpened, {toggle: toggleDrawer, close: closeDrawer}] =
    useDisclosure(false);
  const pathname = usePathname();
  const {user: sessionUser} = useSession();
  const user = propUser !== undefined ? propUser : sessionUser;

  const getReturnUrlDrawer = () => {
    if (!pathname || pathname === "/" || pathname.startsWith("/auth")) {
      return null;
    }
    return pathname;
  };
  const returnUrl = getReturnUrlDrawer();
  const returnTo = returnUrl
    ? `?return_to=${encodeURIComponent(returnUrl)}`
    : "";

  const {setColorScheme} = useMantineColorScheme();
  const computedColorScheme = useComputedColorScheme("dark", {
    getInitialValueInEffect: true,
  });

  const hasSecondaryNav = Boolean(secondaryNav || (secondaryNavItems && secondaryNavItems.length > 0));

  return (
    <>
      <div
        className={cx(
          classes.header,
          hasSecondaryNav && classes.headerWithSecondaryNav,
        )}
      >
        <div className={classes.headerTop}>
          <Group
            h="100%"
            maw="1920px"
            mx="auto"
            wrap="nowrap"
            justify="space-between"
            style={{flex: 1, position: "relative"}}
          >
            <Group
              justify="flex-start"
              h="100%"
              className={classes.leftSection}
              gap="xs"
            >
              <Burger
                opened={drawerOpened}
                onClick={toggleDrawer}
                hiddenFrom="sm"
              />
              <Group gap={6} wrap="nowrap" className={classes.brandingGroup}>
                <Link href="/" className={classes.logoLink}>
                  <Image
                    component={NextImage}
                    src="/gate_logo.svg"
                    alt="Gate logo"
                    width={40}
                    height={40}
                    priority
                    className={classes.logoImage}
                  />
                </Link>

                {organization ? (
                  <>
                    <Link
                      href={`/${organization.login || organization.id}`}
                      className={classes.brandTitleLink}
                      title={organization.name}
                    >
                      <Title order={1} className={classes.brandTitleText}>
                        {organization.name}
                      </Title>
                    </Link>

                    {contest && (
                      <div className={classes.organizationCrumb}>
                        <span className={classes.organizationSlash}>/</span>
                        <Link
                          href={`/${organization.login || organization.id}/${contest.login || contest.id}`}
                          className={classes.organizationLink}
                          title={contest.title}
                        >
                          {contest.title}
                        </Link>
                      </div>
                    )}
                    {!contest && problem && (
                      <div className={classes.organizationCrumb}>
                        <span className={classes.organizationSlash}>/</span>
                        <Link
                          href={`/${organization.login || organization.id}/problems/${problem.id}`}
                          className={classes.organizationLink}
                          title={problem.title}
                        >
                          {problem.title}
                        </Link>
                      </div>
                    )}
                  </>
                ) : (
                  <Link href="/" className={classes.brandTitleLink}>
                    <Title order={1} className={classes.brandTitleText}>
                      Gate
                    </Title>
                  </Link>
                )}
              </Group>
            </Group>

            <Box hiddenFrom="sm" style={{flex: 1}} />
            <Group
              justify="flex-end"
              h="100%"
              gap="xs"
              className={classes.rightSection}
            >
              {user?.role === "admin" && (
                <Button
                  component={Link}
                  href="/admin"
                  variant="filled"
                  visibleFrom="sm"
                  color={APP_COLORS.admin}
                >
                  ADMIN
                </Button>
              )}
              <ActionIcon
                onClick={() =>
                  setColorScheme(
                    computedColorScheme === "light" ? "dark" : "light",
                  )
                }
                variant="default"
                size="input-sm"
                aria-label="Toggle color scheme"
              >
                <IconSun
                  className={cx(classes.icon, classes.light)}
                  stroke={1.5}
                />
                <IconMoon
                  className={cx(classes.icon, classes.dark)}
                  stroke={1.5}
                />
              </ActionIcon>
              <Profile user={user} />
            </Group>
          </Group>
        </div>

        {(() => {
          if (secondaryNav) {
            return (
              <div className={classes.secondaryNavSection}>{secondaryNav}</div>
            );
          }
          if (secondaryNavItems && secondaryNavItems.length > 0) {
            return (
              <div className={classes.secondaryNavSection}>
                <AdaptiveTabs items={secondaryNavItems} />
              </div>
            );
          }
          return null;
        })()}
      </div>

      <Drawer
        opened={drawerOpened}
        onClose={closeDrawer}
        size="100%"
        hiddenFrom="sm"
        zIndex={1000000}
      >
        <ScrollArea h="calc(100vh - 80px)" mx="-md">
          <Stack gap="xs" p="md">
            {user?.role === "admin" && (
              <Anchor
                component={Link}
                href="/admin"
                className={classes.link}
                underline="never"
                onClick={closeDrawer}
              >
                Администрирование
              </Anchor>
            )}

            <Divider my="sm" />

            <Group justify="space-between" align="center">
              <span style={{fontWeight: 600}}>Тема оформления</span>
              <ActionIcon
                onClick={() =>
                  setColorScheme(
                    computedColorScheme === "light" ? "dark" : "light",
                  )
                }
                variant="default"
                size="lg"
                aria-label="Toggle color scheme"
              >
                <IconSun
                  className={cx(classes.icon, classes.light)}
                  stroke={1.5}
                />
                <IconMoon
                  className={cx(classes.icon, classes.dark)}
                  stroke={1.5}
                />
              </ActionIcon>
            </Group>

            <Divider my="sm" />

            {user ? (
              <Stack gap="sm">
                <Button
                  component={Link}
                  href={`/@${user.username}`}
                  variant="light"
                  color={APP_COLORS.users}
                  leftSection={<IconUser size={20} />}
                  fullWidth
                  onClick={closeDrawer}
                >
                  Профиль
                </Button>
                <Button
                  component={Link}
                  href="/orgs"
                  variant="light"
                  color={APP_COLORS.orgs}
                  leftSection={<IconBuilding size={20} />}
                  fullWidth
                  onClick={closeDrawer}
                >
                  Организации
                </Button>
                <LogoutLink variant="outline" fullWidth>
                  Выйти
                </LogoutLink>
              </Stack>
            ) : (
              <Button
                component={Link}
                href={`/auth/login${returnTo}`}
                variant="filled"
                color={APP_COLORS.actions.primary}
                fullWidth
                onClick={closeDrawer}
              >
                Войти
              </Button>
            )}
          </Stack>
        </ScrollArea>
      </Drawer>
    </>
  );
};
