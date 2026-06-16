"use client";

import type { SessionUser } from "@/lib/auth";
import { logoutAction } from "@/lib/auth-actions";
import type {
  HeaderSecondaryNavIcon,
  HeaderSecondaryNavItem,
} from "@/lib/contest-header-nav";
import { APP_COLORS } from "@/lib/theme/colors";
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
  Popover,
  ScrollArea,
  Stack,
  Title,
  useComputedColorScheme,
  useMantineColorScheme,
} from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import {
  IconDeviceDesktop,
  IconLogout,
  IconMail,
  IconMoon,
  IconPuzzle,
  IconSend,
  IconSettings,
  IconSun,
  IconTrophy,
  IconUser,
  IconUsers,
  IconUsersGroup,
} from "@tabler/icons-react";
import cx from "clsx";
import NextImage from "next/image";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  type ComponentType,
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import classes from "./Header.module.css";
import { LogoutLink } from "./LogoutLink";

const NAV_ICON_MAP: Record<
  HeaderSecondaryNavIcon,
  ComponentType<{ size?: string | number }>
> = {
  tasks: IconPuzzle,
  submit: IconSend,
  mysubmissions: IconUser,
  allsubmissions: IconMail,
  monitor: IconDeviceDesktop,
  manage: IconSettings,
  contests: IconTrophy,
  problems: IconPuzzle,
  teams: IconUsersGroup,
  members: IconUsers,
  settings: IconSettings,
};

const useIsomorphicLayoutEffect =
  typeof window === "undefined" ? useEffect : useLayoutEffect;

export type HeaderOrganization = {
  id: string;
  name: string;
};

const Profile = ({ user }: { user?: SessionUser }) => {
  const pathname = usePathname();
  const [logoutLoading, setLogoutLoading] = useState(false);

  const isLocalhost =
    typeof window !== "undefined" &&
    (window.location.hostname === "localhost" ||
      window.location.hostname === "127.0.0.1");
  const returnUrl =
    pathname && pathname !== "/" && !pathname.startsWith("/auth")
      ? isLocalhost
        ? `${window.location.origin}${pathname}`
        : pathname
      : null;
  const returnTo = returnUrl
    ? `?return_to=${encodeURIComponent(returnUrl)}`
    : "";

  const handleLogout = async () => {
    setLogoutLoading(true);
    try {
      await logoutAction();
      window.location.href = "/auth/login";
    } catch {
      window.location.href = "/auth/login";
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

  if (user) {
    return (
      <Group justify="flex-end">
        <Menu
          shadow="md"
          width={200}
          position="bottom-end"
          transitionProps={{ transition: "pop-top-right" }}
        >
          <Menu.Target>
            <Avatar
              color={APP_COLORS.users}
              size={36}
              style={{ cursor: "pointer" }}
            >
              <IconUser size={20} />
            </Avatar>
          </Menu.Target>

          <Menu.Dropdown>
            <Menu.Item
              component={Link}
              href={`/users/${user.id}`}
              leftSection={<IconUser size={16} />}
            >
              Профиль
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

const SecondaryNav = ({ items }: { items: HeaderSecondaryNavItem[] }) => {
  const pathname = usePathname();
  const [visibleCount, setVisibleCount] = useState(items.length);
  const [moreOpened, setMoreOpened] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const moreMeasureRef = useRef<HTMLButtonElement | null>(null);
  const itemMeasureRefs = useRef<Array<HTMLSpanElement | null>>([]);

  const recalculateVisibleCount = useCallback(() => {
    if (items.length === 0) {
      setVisibleCount(0);
      return;
    }

    const containerWidth = containerRef.current?.clientWidth ?? 0;
    if (containerWidth === 0) {
      setVisibleCount(items.length);
      return;
    }

    const itemWidths = items.map(
      (_, index) => itemMeasureRefs.current[index]?.offsetWidth ?? 0,
    );
    if (itemWidths.some((width) => width === 0)) {
      setVisibleCount(items.length);
      return;
    }

    const gap = 8;
    const totalWidth =
      itemWidths.reduce((sum, width) => sum + width, 0) +
      Math.max(0, itemWidths.length - 1) * gap;

    if (totalWidth <= containerWidth) {
      setVisibleCount(items.length);
      return;
    }

    const moreWidth = moreMeasureRef.current?.offsetWidth ?? 72;
    const availableWidth = containerWidth - moreWidth - gap;
    let widthUsed = 0;
    let nextVisibleCount = 0;

    for (const width of itemWidths) {
      const nextWidth = widthUsed + width + (nextVisibleCount > 0 ? gap : 0);
      if (nextWidth > availableWidth) {
        break;
      }

      widthUsed = nextWidth;
      nextVisibleCount += 1;
    }

    setVisibleCount(Math.max(0, nextVisibleCount));
  }, [items]);

  useIsomorphicLayoutEffect(() => {
    recalculateVisibleCount();
  }, [recalculateVisibleCount]);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) {
      return;
    }

    const observer = new ResizeObserver(() => {
      recalculateVisibleCount();
    });
    observer.observe(container);

    return () => {
      observer.disconnect();
    };
  }, [recalculateVisibleCount]);

  useEffect(() => {
    if (!document.fonts?.ready) {
      return;
    }

    let mounted = true;
    document.fonts.ready.then(() => {
      if (mounted) {
        recalculateVisibleCount();
      }
    });

    return () => {
      mounted = false;
    };
  }, [recalculateVisibleCount]);

  useEffect(() => {
    setMoreOpened(false);
  }, [pathname]);

  const visibleItems = items.slice(0, visibleCount);
  const overflowItems = items.slice(visibleCount);

  return (
    <div className={classes.secondaryNavSection}>
      <div className={classes.secondaryNavInner} ref={containerRef}>
        <div className={classes.secondaryNavVisible}>
          {visibleItems.map((item) => {
            const Icon = item.icon ? NAV_ICON_MAP[item.icon] : undefined;

            return (
              <Link
                key={item.key}
                href={item.href}
                className={cx(
                  classes.secondaryNavLink,
                  item.active && classes.secondaryNavLinkActive,
                )}
              >
                {Icon ? <Icon size={15} /> : null}
                {item.label}
              </Link>
            );
          })}
        </div>

        {overflowItems.length > 0 && (
          <Popover
            opened={moreOpened}
            onChange={setMoreOpened}
            position="bottom-end"
            withArrow
            shadow="md"
          >
            <Popover.Target>
              <button
                type="button"
                className={classes.secondaryMoreButton}
                onClick={() => setMoreOpened((current) => !current)}
                aria-haspopup="menu"
                aria-expanded={moreOpened}
                data-opened={moreOpened || undefined}
              >
                More
              </button>
            </Popover.Target>
            <Popover.Dropdown p="xs">
              <Stack gap={4}>
                {overflowItems.map((item) => {
                  const Icon = item.icon ? NAV_ICON_MAP[item.icon] : undefined;

                  return (
                    <Link
                      key={item.key}
                      href={item.href}
                      className={cx(
                        classes.secondaryNavPopoverLink,
                        item.active && classes.secondaryNavPopoverLinkActive,
                      )}
                      onClick={() => setMoreOpened(false)}
                    >
                      {Icon ? <Icon size={15} /> : null}
                      {item.label}
                    </Link>
                  );
                })}
              </Stack>
            </Popover.Dropdown>
          </Popover>
        )}
      </div>

      <div className={classes.secondaryNavMeasure} aria-hidden>
        {items.map((item, index) => {
          const Icon = item.icon ? NAV_ICON_MAP[item.icon] : undefined;

          return (
            <span
              key={`measure-${item.key}`}
              ref={(node) => {
                itemMeasureRefs.current[index] = node;
              }}
              className={classes.secondaryNavLink}
            >
              {Icon ? <Icon size={15} /> : null}
              {item.label}
            </span>
          );
        })}
        <button
          ref={moreMeasureRef}
          type="button"
          className={classes.secondaryMoreButton}
        >
          More
        </button>
      </div>
    </div>
  );
};

const Header = ({
  user,
  secondaryNavItems,
  organization,
}: {
  user?: SessionUser;
  secondaryNavItems?: HeaderSecondaryNavItem[];
  organization?: HeaderOrganization;
}) => {
  const [drawerOpened, { toggle: toggleDrawer, close: closeDrawer }] =
    useDisclosure(false);
  const pathname = usePathname();
  const isLocalhost =
    typeof window !== "undefined" &&
    (window.location.hostname === "localhost" ||
      window.location.hostname === "127.0.0.1");
  const returnUrl =
    pathname && pathname !== "/" && !pathname.startsWith("/auth")
      ? isLocalhost
        ? `${window.location.origin}${pathname}`
        : pathname
      : null;
  const returnTo = returnUrl
    ? `?return_to=${encodeURIComponent(returnUrl)}`
    : "";

  const { setColorScheme } = useMantineColorScheme();
  const computedColorScheme = useComputedColorScheme("dark", {
    getInitialValueInEffect: true,
  });
  const hasSecondaryNav = Boolean(secondaryNavItems?.length);

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
            style={{ flex: 1, position: "relative" }}
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
                  <Group gap="xs" wrap="nowrap">
                    <Image
                      component={NextImage}
                      src="/gate_logo.svg"
                      alt="Gate logo"
                      width={40}
                      height={40}
                      priority
                      className={classes.logoImage}
                    />
                    <Title order={1}>Gate</Title>
                  </Group>
                </Link>

                {organization && (
                  <div className={classes.organizationCrumb}>
                    <span className={classes.organizationSlash}>/</span>
                    <Link
                      href={`/orgs/${organization.id}`}
                      className={classes.organizationLink}
                      title={organization.name}
                    >
                      {organization.name}
                    </Link>
                  </div>
                )}
              </Group>
            </Group>
            <Group
              justify="center"
              h="100%"
              gap={0}
              visibleFrom="sm"
              style={{
                position: "absolute",
                left: "50%",
                transform: "translateX(-50%)",
              }}
            >
              <Anchor
                component={Link}
                href="/"
                className={classes.link}
                underline="never"
              >
                Главная
              </Anchor>
              <Anchor
                component={Link}
                href="/orgs"
                className={classes.link}
                underline="never"
              >
                Организации
              </Anchor>
            </Group>
            <Box hiddenFrom="sm" style={{ flex: 1 }} />
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

        {secondaryNavItems && secondaryNavItems.length > 0 && (
          <SecondaryNav items={secondaryNavItems} />
        )}
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
            <Anchor
              component={Link}
              href="/"
              className={classes.link}
              underline="never"
              onClick={closeDrawer}
            >
              Главная
            </Anchor>
            <Anchor
              component={Link}
              href="/orgs"
              className={classes.link}
              underline="never"
              onClick={closeDrawer}
            >
              Организации
            </Anchor>

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
              <span style={{ fontWeight: 600 }}>Тема оформления</span>
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
                  href={`/users/${user.id}`}
                  variant="light"
                  color={APP_COLORS.users}
                  leftSection={<IconUser size={20} />}
                  fullWidth
                  onClick={closeDrawer}
                >
                  Профиль
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

export { Header };
