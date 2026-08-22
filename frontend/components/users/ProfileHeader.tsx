"use client";

import {Avatar, Badge, Group, Paper, Stack, Text, Title} from "@mantine/core";
import {IconCalendar} from "@tabler/icons-react";

import {APP_COLORS} from "@/lib/theme/colors";

import type {ReactNode} from "react";

const ROLE_COLORS: Record<string, string> = {
  admin: "red",
  moderator: "blue",
};

type ProfileHeaderProps = {
  username: string;
  role: string;
  createdAt?: string;
  expiresAt?: string | null;
  claimedAt?: string | null;
  isOwnProfile?: boolean;
};

export const ProfileHeader = (props: ProfileHeaderProps): ReactNode => {
  const showRole = props.role?.toLowerCase() !== "user";
  const roleColor = ROLE_COLORS[props.role?.toLowerCase()] ?? "gray";
  const initials = props.username?.[0]?.toUpperCase() ?? "?";

  const isExpired = props.expiresAt ? new Date(props.expiresAt) < new Date() : false;

  return (
    <Paper shadow="sm" p="lg" radius="md">
      <Group align="flex-start" gap="xl">
        <Avatar size={72} radius="xl" color={APP_COLORS.users}>
          {initials}
        </Avatar>
        <Stack gap="xs" style={{flex: 1}}>
          <Group gap="sm" align="center" justify="space-between">
            <Group gap="sm" align="center">
              <Title order={2}>@{props.username}</Title>
              {showRole && (
                <Badge color={roleColor} size="lg">
                  {props.role}
                </Badge>
              )}
              {props.expiresAt && (
                <Badge color={isExpired ? "red" : "orange"} variant="light" size="md">
                  {isExpired
                    ? `Срок истёк (${new Date(props.expiresAt).toLocaleDateString("ru-RU")})`
                    : `Временный (до ${new Date(props.expiresAt).toLocaleDateString("ru-RU")})`}
                </Badge>
              )}
              {props.claimedAt && (
                <Badge color="teal" variant="light" size="md">
                  Привязан к профилю
                </Badge>
              )}
            </Group>
          </Group>
          <Group gap="lg">
            {props.createdAt && (
              <Group gap="xs">
                <IconCalendar size={14} style={{color: "var(--mantine-color-dimmed)"}} />
                <Text size="sm" c="dimmed">
                  На платформе с{" "}
                  {new Date(props.createdAt!).toLocaleDateString("ru-RU", {
                    day: "2-digit",
                    month: "long",
                    year: "numeric",
                  })}
                </Text>
              </Group>
            )}
          </Group>
        </Stack>
      </Group>
    </Paper>
  );
};
