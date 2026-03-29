"use client";

import { Alert, Button, FileInput, Group, ScrollArea, Stack, Text } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { IconInfoCircle, IconUpload } from "@tabler/icons-react";
import { useRouter } from "next/navigation";
import { useState, useTransition } from "react";
import { SectionPaper } from "@/components/workshop/SectionPaper";
import { importProblemPackage } from "@/lib/actions";

type Props = {
  problemId: string;
};

export function WorkshopImportTab({ problemId }: Props) {
  const router = useRouter();
  const [packageFile, setPackageFile] = useState<File | null>(null);
  const [isImporting, startImport] = useTransition();

  const handleImport = () => {
    if (!packageFile) return;

    startImport(async () => {
      const [error] = await importProblemPackage(problemId, packageFile);
      if (error) {
        notifications.show({
          title: "Ошибка импорта",
          message: error.message ?? "Не удалось импортировать пакет",
          color: "red",
        });
        return;
      }

      notifications.show({
        title: "Пакет импортирован",
        message: "Воркшоп обновлен из загруженного архива",
        color: "green",
      });
      setPackageFile(null);
      router.refresh();
    });
  };

  return (
    <ScrollArea style={{ flex: 1 }} p="lg">
      <Stack gap="lg" maw={900} mx="auto">
        <SectionPaper title="Импорт пакета">
          <Stack gap="sm">
            <Text size="sm" c="dimmed">
              Загрузите zip-архив задачи для импорта в текущий воркшоп. Поддерживаются форматы
              ICPC и Polygon, а также native-пакеты Gate.
            </Text>

            <Alert color="blue" variant="light" icon={<IconInfoCircle size={16} />}>
              Импорт перезапишет файлы воркшопа для этой задачи.
            </Alert>

            <Group align="flex-end" wrap="wrap">
              <FileInput
                label="Архив пакета"
                placeholder="Выберите .zip файл"
                accept=".zip,application/zip"
                value={packageFile}
                onChange={setPackageFile}
                style={{ minWidth: 320, flex: 1 }}
              />
              <Button
                leftSection={<IconUpload size={16} />}
                onClick={handleImport}
                disabled={!packageFile}
                loading={isImporting}
              >
                Импортировать
              </Button>
            </Group>
          </Stack>
        </SectionPaper>
      </Stack>
    </ScrollArea>
  );
}
