"use client";

import { Alert, Button, Group, Modal, Radio, Stack, Text } from "@mantine/core";
import { useState } from "react";

type Props = {
  opened: boolean;
  onClose: () => void;
  subtaskName: string;
  testCount: number;
  onConfirm: (deleteTests: boolean) => void;
};

export const SubtaskDeleteModal = ({
  opened,
  onClose,
  subtaskName,
  testCount,
  onConfirm,
}: Props) => {
  const [deleteTests, setDeleteTests] = useState<"keep" | "delete">("keep");

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title={<Text fw={600}>Удаление сабтаска &quot;{subtaskName}&quot;</Text>}
      size="md"
    >
      <Stack gap="md">
        <Text size="sm">
          В этом сабтаске находится тестов: <b>{testCount}</b>.Выберите действие:
        </Text>

        <Radio.Group
          value={deleteTests}
          onChange={(v) => setDeleteTests(v as "keep" | "delete")}
        >
          <Stack gap="sm" mt="xs">
            <Radio
              value="keep"
              label="Удалить только сабтаск"
              description="Тесты останутся в списке задач как нераспределенные."
            />
            <Radio
              value="delete"
              label="Удалить сабтаск вместе со входящими в него тестами"
              description="Тесты и относящиеся к ним файлы входных и выходных данных будут удалены."
            />
          </Stack>
        </Radio.Group>

        {deleteTests === "delete" && (
          <Alert color="red" title="Внимание!">
            Файлы тестов будут безвозвратно удалены!
          </Alert>
        )}

        <Group justify="flex-end" gap="sm" mt="md">
          <Button variant="default" onClick={onClose}>
            Отмена
          </Button>

          <Button
            color="red"
            onClick={() => {
              onConfirm(deleteTests === "delete");
              onClose();
            }}
          >
            Удалить сабтаск
          </Button>
        </Group>
      </Stack>
    </Modal>
  );
};
