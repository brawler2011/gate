"use client";

import { APP_COLORS } from "@/lib/theme/colors";
import {
  ActionIcon,
  Anchor,
  Avatar,
  Box,
  Burger,
  Button,
  Divider,
  Drawer,
  Group,
  Image,
  ScrollArea,
  Stack,
  Title,
  useComputedColorScheme,
  useMantineColorScheme,
} from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { IconMoon, IconSun, IconUser } from "@tabler/icons-react";
import type { SessionUser } from "@/lib/auth";
import cx from "clsx";
import NextImage from "next/image";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { LogoutLink } from './LogoutLink';
import classes from "./Header.module.css";

const Profile = ({ user }: { user?: SessionUser }) => {
  const pathname = usePathname();
  const isLocalhost = typeof window !== 'undefined' && (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1');
  const returnUrl = pathname && pathname !== '/' && !pathname.startsWith('/auth') 
    ? (isLocalhost ? `${window.location.origin}${pathname}` : pathname) 
    : null;
  const returnTo = returnUrl ? `?return_to=${encodeURIComponent(returnUrl)}` : '';

  if (user) {
    return (
      <Group justify="flex-end">
        <LogoutLink variant="default" visibleFrom="sm">
          Выйти
        </LogoutLink>
        <Avatar
          component={Link}
          href={`/users/${user.id}`}
          color={APP_COLORS.users}
          size="60"
        >
          <IconUser size="32" />
        </Avatar>
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
      >
        Войти
      </Button>
    </Group>
  );
};

const Header = ({ user }: { user?: SessionUser }) => {
  const [drawerOpened, { toggle: toggleDrawer, close: closeDrawer }] = useDisclosure(false);
  const pathname = usePathname();
  const isLocalhost = typeof window !== 'undefined' && (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1');
  const returnUrl = pathname && pathname !== '/' && !pathname.startsWith('/auth') 
    ? (isLocalhost ? `${window.location.origin}${pathname}` : pathname) 
    : null;
  const returnTo = returnUrl ? `?return_to=${encodeURIComponent(returnUrl)}` : '';

  const { setColorScheme } = useMantineColorScheme();
  const computedColorScheme = useComputedColorScheme("dark", {
    getInitialValueInEffect: true,
  });

  return (
    <>
      <div className={classes.header}>
        <Group h="100%" maw="1920px" mx="auto" wrap="nowrap" justify="space-between" style={{ flex: 1, position: "relative" }}>
          <Group justify="flex-start" h="100%" className={classes.leftSection} gap="xs">
            <Burger
              opened={drawerOpened}
              onClick={toggleDrawer}
              hiddenFrom="sm"
            />
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
                <Title order={1}>
                  Gate
                </Title>
              </Group>
            </Link>
          </Group>
          <Group justify="center" h="100%" gap={0} visibleFrom="sm" style={{ position: "absolute", left: "50%", transform: "translateX(-50%)" }}>
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
              href="/contests"
              className={classes.link}
              underline="never"
            >
              Контесты
            </Anchor>
            <Anchor
              component={Link}
              href="/workshop"
              className={classes.link}
              underline="never"
            >
              Мастерская
            </Anchor>
            <Anchor
              component={Link}
              href="/about"
              className={classes.link}
              underline="never"
            >
              О платформе
            </Anchor>
          </Group>
          <Box hiddenFrom="sm" style={{ flex: 1 }} />
          <Group justify="flex-end" h="100%" gap="xs" className={classes.rightSection}>
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
                  computedColorScheme === "light" ? "dark" : "light"
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
              href="/contests"
              className={classes.link}
              underline="never"
              onClick={closeDrawer}
            >
              Контесты
            </Anchor>
            <Anchor
              component={Link}
              href="/workshop"
              className={classes.link}
              underline="never"
              onClick={closeDrawer}
            >
              Мастерская
            </Anchor>
            <Anchor
              component={Link}
              href="/about"
              className={classes.link}
              underline="never"
              onClick={closeDrawer}
            >
              О платформе
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
                    computedColorScheme === "light" ? "dark" : "light"
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
