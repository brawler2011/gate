"use client";

import { useTransition } from "react";
import { createProblem } from "@/lib/actions";
import { Button } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { IconPlus } from "@tabler/icons-react";
import { useRouter } from "next/navigation";

const CreateProblemForm = () => {
  const router = useRouter();
  const [isPending, startTransition] = useTransition();

  const handleCreate = () => {
    startTransition(async () => {
      try {
        const [error, response] = await createProblem("New Problem");
        if (error) {
          notifications.show({
            title: "Ошибка",
            message: error.message,
            color: "red",
          });
          return;
        }
        if (!response?.id) {
          notifications.show({
            title: "Ошибка",
            message: "Не получен ID задачи",
            color: "red",
          });
          return;
        }
        router.push(`/problems/${response.id}`);
      } catch (error) {
        console.error("Не удалось создать задачу. Попробуйте позже.", error);
        notifications.show({
          title: "Ошибка",
          message:
            error instanceof Error ? error.message : "Не удалось создать задачу",
          color: "red",
        });
      }
    });
  };

  return (
    <Button
      title="Создать новую задачу"
      onClick={handleCreate}
      loading={isPending}
      size="md"
      variant="light"
      leftSection={<IconPlus size={18} />}
      fullWidth
    >
      Создать задачу
    </Button>
  );
};

export { CreateProblemForm };
