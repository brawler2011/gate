"use client";

import { createProblem } from "@/lib/actions";
import { Button } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { IconPlus } from "@tabler/icons-react";
import { useRouter } from "next/navigation";
import useSWRMutation from "swr/mutation";

const CreateProblemForm = () => {
  const router = useRouter();

  const { trigger, isMutating } = useSWRMutation(
    "createProblem",
    async (): Promise<string> => {
      const [error, response] = await createProblem("New Problem");
      if (error) throw new Error(error.message);
      if (!response?.id) throw new Error("Не получен ID задачи");
      return response.id;
    },
    {
      onSuccess: (problemId: string) => {
        router.push(`/problems/${problemId}`);
      },
      onError: (error) => {
        console.error("Не удалось создать задачу. Попробуйте позже.", error);
        notifications.show({
          title: "Ошибка",
          message:
            error instanceof Error ? error.message : "Не удалось создать задачу",
          color: "red",
        });
      },
    }
  );
  return (
    <Button
      title="Создать новую задачу"
      onClick={() => trigger()}
      loading={isMutating}
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
