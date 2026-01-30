import { Avatar, Badge, Card, Container, Group, Paper, Stack, Text, Title } from "@mantine/core";
import { IconUser, IconMail } from "@tabler/icons-react";
import { getRoleColor } from "@/lib/lib";
import { APP_COLORS } from "@/lib/theme/colors";

type ProfileProps = {
  username: string;
  email?: string;
  name?: string;
  surname?: string;
  role: string;
  avatarlink?: string;
};

const Profile = (props: ProfileProps) => {
  const showRole = props.role?.toLowerCase() !== "user";

  return (
    <Container size="sm" px={0}>
      <Paper shadow="sm" p="xl" radius="md">
        <Stack gap="xl">
        {/* Avatar and Title Section */}
        <Group align="flex-start" gap="lg">
          <Avatar 
            src={props.avatarlink} 
            size={128} 
            radius="xl"
            color={APP_COLORS.users}
          >
            <IconUser size={64} />
          </Avatar>
          <Stack gap="xs" style={{ flex: 1 }}>
            <Title order={1}>@{props.username}</Title>
            {(props.name || props.surname) || (
              <Title order={2} c="dimmed" fw={400}>
                {/* {[props.name, props.surname].filter(Boolean).join(' ') || */"Алексей Котоков" }
              </Title>
            )}
            {showRole && (
              <Badge color={getRoleColor(props.role)} size="lg">
                {props.role}
              </Badge>
            )}
          </Stack>
        </Group>

        {/* Email Section */}
        {props.email || (
          <Card withBorder padding="md" radius="md">
            <Group gap="sm">
              <IconMail size={20} style={{ color: `var(--mantine-color-${APP_COLORS.users}-6)` }} />
              <Text size="sm" fw={500}>
                {props.email || "kotok.9647@gmail.com"}
              </Text>
            </Group>
          </Card>
        )}

        {/* Statistics Section */}
        <Card withBorder padding="md" radius="md">
          <Title order={3} mb="md">
            Статистика решений
          </Title>
          <Stack gap="md">
            <Group gap="sm">
              <Text size="sm" c="dimmed" style={{ minWidth: "170px" }}>
                Всего решений:
              </Text>
              <Text size="sm" fw={500}>
                0
              </Text>
            </Group>
            <Group gap="sm">
              <Text size="sm" c="dimmed" style={{ minWidth: "170px" }}>
                Успешных решений:
              </Text>
              <Text size="sm" fw={500}>
                0
              </Text>
            </Group>
          </Stack>
        </Card>
      </Stack>
    </Paper>
    </Container>
  );
};

export { Profile };
