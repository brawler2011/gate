"use client";

import {Avatar, Badge, Group, Paper, Stack, Text, Title} from "@mantine/core";
import {IconCalendar} from "@tabler/icons-react";

import {getRoleColor} from "@/lib/lib";
import {APP_COLORS} from "@/lib/theme/colors";

import type {ReactNode} from "react";

type ProfileHeaderProps = {
  username: string;
  role: string;
  createdAt?: string;
  isOwnProfile?: boolean;
};

export const ProfileHeader = (props: ProfileHeaderProps): ReactNode => {
  const showRole = props.role?.toLowerCase() !== "user";
  const initials = props.username?.[0]?.toUpperCase() ?? "?";

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
                <Badge color={getRoleColor(props.role)} size="lg">
                  {props.role}
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
