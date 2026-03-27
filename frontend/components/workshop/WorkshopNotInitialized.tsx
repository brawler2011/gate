"use client";

import { Button, Center, Stack, Text, Title } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { IconFolderPlus } from "@tabler/icons-react";
import { useRouter } from "next/navigation";
import { useTransition } from "react";
import { initProblemWorkshop } from "@/lib/actions";

type Props = {
  problemId: string;
};

export function WorkshopNotInitialized({ problemId }: Props) {
  const router = useRouter();
  const [isPending, startTransition] = useTransition();

  const handleInit = () => {
    startTransition(async () => {
      const [error] = await initProblemWorkshop(problemId);
      if (error) {
        notifications.show({
          title: "Ошибка инициализации",
          message: error.message ?? "Не удалось инициализировать воркшоп",
          color: "red",
        });
        return;
      }
      notifications.show({
        title: "Воркшоп создан",
        message: "Рабочее пространство задачи успешно инициализировано",
        color: "green",
      });
      router.refresh();
    });
  };

  return (
    <Center style={{ flex: 1, height: "calc(100vh - 120px)" }}>
      <Stack align="center" gap="md">
        <IconFolderPlus size={48} color="var(--mantine-color-dimmed)" />
        <Title order={3} c="dimmed">
          Воркшоп не инициализирован
        </Title>
        <Text size="sm" c="dimmed" ta="center" maw={360}>
          Для этой задачи ещё не создано рабочее пространство. Нажмите кнопку
          ниже, чтобы создать начальную структуру файлов.
        </Text>
        <Button loading={isPending} onClick={handleInit}>
          Инициализировать воркшоп
        </Button>
      </Stack>
    </Center>
  );
}
