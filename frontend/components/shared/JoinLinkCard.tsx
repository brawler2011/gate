import {Badge, Button, Card, CopyButton, Group, Stack, Text, TextInput} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {useMutation, useQuery, useQueryClient} from "@tanstack/react-query";

import type {JoinLinkModel} from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

type Props = {
  queryKey: readonly unknown[];
  load: () => Promise<JoinLinkModel>;
  reset: () => Promise<JoinLinkModel>;
  disable: () => Promise<void>;
  description: string;
};

export const JoinLinkCard = ({queryKey, load, reset, disable, description}: Props): ReactNode => {
  const queryClient = useQueryClient();
  const link = useQuery({queryKey, queryFn: load});
  const onError = (error: Error) => notifications.show({title: "Ошибка", message: error.message, color: "red"});
  const resetLink = useMutation({
    mutationFn: reset,
    onSuccess: (data) => queryClient.setQueryData(queryKey, data),
    onError,
  });
  const disableLink = useMutation({
    mutationFn: disable,
    onSuccess: () => queryClient.setQueryData(queryKey, {}),
    onError,
  });
  const token = link.data?.token;
  const url = token ? `${window.location.origin}/join/${token}` : "";

  return (
    <Card withBorder>
      <Stack gap="sm">
        <Group justify="space-between">
          <Text fw={600}>Ссылка для вступления</Text>
          <Badge color={token ? "green" : "gray"} variant="light">{token ? "Включена" : "Выключена"}</Badge>
        </Group>
        <Text size="sm" c="dimmed">{description}</Text>
        {token ? (
          <>
            <Group gap="xs" wrap="nowrap">
              <TextInput value={url} readOnly style={{flex: 1}} aria-label="Ссылка для вступления" />
              <CopyButton value={url}>
                {({copied, copy}) => (
                  <Button variant="light" color={copied ? "teal" : "blue"} onClick={copy}>
                    {copied ? "Скопировано" : "Копировать"}
                  </Button>
                )}
              </CopyButton>
            </Group>
            <Group gap="xs">
              <Button variant="subtle" size="xs" onClick={() => resetLink.mutate()} loading={resetLink.isPending}>
                Заменить ссылку
              </Button>
              <Button variant="subtle" size="xs" color="red" onClick={() => disableLink.mutate()} loading={disableLink.isPending}>
                Выключить
              </Button>
            </Group>
            <Text size="xs" c="dimmed">Если ссылка попала не туда, замените её: старая перестанет работать.</Text>
          </>
        ) : (
          <Group>
            <Button onClick={() => resetLink.mutate()} loading={resetLink.isPending} disabled={link.isPending}>
              Включить ссылку
            </Button>
          </Group>
        )}
      </Stack>
    </Card>
  );
};
