"use client";

import { createContest } from "@/lib/actions";
import { Button } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { IconPlus } from "@tabler/icons-react";
import { useMutation } from "@tanstack/react-query";
import { useRouter } from "next/navigation";

const CreateContestForm = () => {
  const router = useRouter();

  const mutation = useMutation({
    mutationFn: async () => {
      const [error, response] = await createContest("New Contest");
      if (error) throw new Error(error.message);
      if (!response?.id) throw new Error("Не получен ID контеста");
      return response.id;
    },
    onSuccess: (contestId: string) => {
      router.push(`/contests/${contestId}`);
    },
    onError: (error) => {
      console.error("Не удалось создать контест. Попробуйте позже.", error);
      notifications.show({
        title: "Ошибка",
        message:
          error instanceof Error ? error.message : "Не удалось создать контест",
        color: "red",
      });
    },
  });

  return (
    <Button
      title="Создать новый контест"
      onClick={() => mutation.mutate()}
      loading={mutation.isPending}
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
