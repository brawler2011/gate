"use client";

import { Button } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { IconPlus } from "@tabler/icons-react";
import { useRouter } from "next/navigation";
import { useTransition } from "react";

import { api } from "@/lib/api";

const CreateContestForm = () => {
  const router = useRouter();
  const [isPending, startTransition] = useTransition();

  const handleCreate = () => {
    startTransition(async () => {
      try {
        const [error, response] = await api.createContest({ title: "New Contest" });
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
            message: "Не получен ID контеста",
            color: "red",
          });
          return;
        }
        router.push(`/contests/${response.id}`);
      } catch (error) {
        console.error("Не удалось создать контест. Попробуйте позже.", error);
        notifications.show({
          title: "Ошибка",
          message:
            error instanceof Error ? error.message : "Не удалось создать контест",
          color: "red",
        });
      }
    });
  };

  return (
    <Button
      title="Создать новый контест"
      onClick={handleCreate}
      loading={isPending}
      size="md"
      variant="light"
      leftSection={<IconPlus size={18} />}
      fullWidth
    >
      Создать контест
    </Button>
  );
};

export { CreateContestForm };
