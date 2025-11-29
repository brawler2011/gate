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
import type { Session } from "@ory/client";
import cx from "clsx";
import NextImage from "next/image";
import Link from "next/link";
import { LogoutLink } from "../LogoutLink";
import classes from "./styles.module.css";

const Profile = ({ session }: { session?: Session | null }) => {
  if (session) {
    return (
      <Group justify="flex-end">
        <LogoutLink variant="default" visibleFrom="sm">
          Выйти
        </LogoutLink>
        {session.identity ? (
          <Avatar
            component={Link}
            href={`/users/${session.identity.metadata_public.user_id}`}
            color={APP_COLORS.users}
            size="60"
          >
            <IconUser size="32" />
          </Avatar>
        ) : (
          <Avatar color={APP_COLORS.users} size="60">
            <IconUser size="32" />
          </Avatar>
        )}
      </Group>
    );
  }
  return (
    <Group justify="flex-end">
      <Button
        component={Link}
        href="/auth/login"
        variant="filled"
        color={APP_COLORS.actions.primary}
      >
        Войти
      </Button>
    </Group>
  );
};

const Header = ({ session }: { session?: Session | null }) => {
  const [drawerOpened, { toggle: toggleDrawer, close: closeDrawer }] =
    useDisclosure(false);

  const { setColorScheme } = useMantineColorScheme();
  const computedColorScheme = useComputedColorScheme("dark", {
    getInitialValueInEffect: true,
  });

  return (
    <>
      <div className={classes.header}>
        <Group h="100%" maw="1920px" mx="auto" wrap="nowrap" style={{ flex: 1 }}>
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
                  className={classes.logoImage}
                />
                <Title order={1}>
                  Gate
                </Title>
              </Group>
            </Link>
          </Group>
          <Group justify="center" h="100%" gap={0} visibleFrom="sm" style={{ flex: 1 }}>
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
            {session?.identity?.metadata_public?.role === "admin" && (
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
            <Profile session={session} />
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
            {session?.identity?.metadata_public?.role === "admin" && (
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

            {session ? (
              <Stack gap="sm">
                {session.identity && (
                  <Button
                    component={Link}
                    href={`/users/${session.identity.metadata_public.user_id}`}
                    variant="light"
                    color={APP_COLORS.users}
                    leftSection={<IconUser size={20} />}
                    fullWidth
                    onClick={closeDrawer}
                  >
                    Профиль
                  </Button>
                )}
                <LogoutLink variant="outline" fullWidth>
                  Выйти
                </LogoutLink>
              </Stack>
            ) : (
              <Button
                component={Link}
                href="/auth/login"
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
