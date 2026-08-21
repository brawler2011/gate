"use client";

import {
  Button,
  Center,
  Group,
  Loader,
  Modal,
  Stack,
  Text,
  Tooltip,
} from "@mantine/core";
import {IconExternalLink} from "@tabler/icons-react";
import Link from "next/link";
import React from "react";
import useSWR from "swr";

import {api} from "@/lib/api";

import {SubmissionDetailsContent} from "./SubmissionDetailsContent";

import type {ReactNode} from "react";

interface SubmissionDetailsModalProps {
  submissionId: string | null;
  opened: boolean;
  onClose: () => void;
  canRejudge?: boolean;
  onRejudge?: (submissionId: string) => Promise<void>;
  isRejudging?: boolean;
}

export const SubmissionDetailsModal = ({
  submissionId,
  opened,
  onClose,
  canRejudge,
  onRejudge,
  isRejudging,
}: SubmissionDetailsModalProps): ReactNode => {
  const {data, error, isLoading} = useSWR(
    opened && submissionId ? ["submission-details", submissionId] : null,
    async () => {
      if (!submissionId) {
        return null;
      }
      const [err, res] = await api.getSubmission({submissionId});
      if (err) {
        throw new Error(err.message || "Не удалось загрузить данные посылки");
      }
      return res?.submission ?? null;
    }
  );

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      size="xl"
      title={
        <Group justify="space-between" w="100%">
          <Text fw={600} size="lg">
            {submissionId ? `Посылка #${submissionId.slice(0, 8)}...` : "Детали посылки"}
          </Text>
          {submissionId && (
            <Tooltip label="Открыть на отдельной странице">
              <Button
                component={Link}
                href={`/submissions/${submissionId}`}
                target="_blank"
                size="xs"
                variant="subtle"
                color="blue"
                rightSection={<IconExternalLink size={14} />}
              >
                На страницу посылки
              </Button>
            </Tooltip>
          )}
        </Group>
      }
      styles={{
        title: {width: "100%"},
        header: {paddingRight: "var(--mantine-spacing-md)"},
      }}
    >
      {isLoading && (
        <Center py="xl">
          <Stack align="center" gap="sm">
            <Loader size="md" />
            <Text size="sm" c="dimmed">
              Загрузка протокола тестирования...
            </Text>
          </Stack>
        </Center>
      )}

      {error && !isLoading && (
        <Center py="xl">
          <Text c="red" size="sm">
            {error.message || "Ошибка загрузки данных посылки"}
          </Text>
        </Center>
      )}

      {data && !isLoading && (
        <SubmissionDetailsContent
          submission={data}
          canRejudge={canRejudge}
          onRejudge={onRejudge}
          isRejudging={isRejudging}
        />
      )}
    </Modal>
  );
};
