"use client";

import {Button, Modal, Text} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconRefresh} from "@tabler/icons-react";
import {useRouter} from "next/navigation";
import React, {useState} from "react";

import {api} from "@/lib/api";

interface SingleSubmissionRejudgeButtonProps {
  contestId: string;
  submissionId: string;
}

export const SingleSubmissionRejudgeButton: React.FC<SingleSubmissionRejudgeButtonProps> = ({
  contestId,
  submissionId,
}) => {
  const [opened, setOpened] = useState(false);
  const [loading, setLoading] = useState(false);
  const router = useRouter();

  const handleRejudge = async () => {
    setLoading(true);
    const [err] = await api.rejudgeSubmission({
      contestId,
      submissionId,
    });
    setLoading(false);
    setOpened(false);

    if (err) {
      notifications.show({
        title: "Ошибка",
        message: err.message || "Не удалось отправить посылку на перетестирование",
        color: "red",
      });
    } else {
      notifications.show({
        title: "Успешно",
        message: "Посылка отправлена на повторную проверку",
        color: "green",
      });
      router.refresh();
    }
  };

  return (
    <>
      <Button
        size="xs"
        variant="outline"
        color="blue"
        leftSection={<IconRefresh size="0.9rem" />}
        onClick={() => setOpened(true)}
      >
        Перетестировать
      </Button>

      <Modal
        opened={opened}
        onClose={() => setOpened(false)}
        title="Перетестирование посылки"
        centered
      >
        <Text size="sm" mb="lg">
          Вы действительно хотите отправить данную посылку на повторную проверку?
        </Text>
        <div style={{display: "flex", justifyContent: "flex-end", gap: "8px"}}>
          <Button variant="default" onClick={() => setOpened(false)} disabled={loading}>
            Отмена
          </Button>
          <Button color="blue" onClick={handleRejudge} loading={loading}>
            Перетестировать
          </Button>
        </div>
      </Modal>
    </>
  );
};
