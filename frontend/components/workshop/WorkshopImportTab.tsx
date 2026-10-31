import {Alert, Box, Button, FileInput, Group, Stack, Text} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconInfoCircle, IconUpload} from "@tabler/icons-react";
import {useQueryClient} from "@tanstack/react-query";
import {useState, useTransition} from "react";

import {SectionPaper} from "@/components/workshop/SectionPaper";
import {api} from "@/lib/api";

import type {ReactNode} from "react";

type Props = {
  problemId: string;
};

export const WorkshopImportTab = ({problemId}: Props): ReactNode => {
  const queryClient = useQueryClient();
  const [packageFile, setPackageFile] = useState<File | null>(null);
  const [isImporting, startImport] = useTransition();

  const handleImport = () => {
    if (!packageFile) {
      return;
    }

    startImport(async () => {
      const body = new FormData();
      body.append("package", packageFile);
      const {error} = await api.POST("/problems/{id}/import", {
        params: {path: {id: problemId}},
        body: body as unknown as {package?: string},
      });
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
      void queryClient.invalidateQueries({
        queryKey: api.queryOptions("get", "/problems/{id}", {
          params: {path: {id: problemId}},
        }).queryKey,
      });
      void queryClient.invalidateQueries({queryKey: ["problem", problemId]});
      void queryClient.invalidateQueries({queryKey: ["problem-limits", problemId]});
      void queryClient.invalidateQueries({queryKey: ["workshop-files", problemId]});
      void queryClient.invalidateQueries({queryKey: ["workshop-single-component-files", problemId]});
    });
  };

  return (
    <Box p="lg">
      <Stack gap="lg" maw={900} mx="auto">
        <SectionPaper title="Импорт пакета">
          <Stack gap="sm">
            <Text size="sm" c="dimmed">
              Загрузите zip-архив задачи для импорта в текущий воркшоп.
              Поддерживаются форматы ICPC и Polygon, а также native-пакеты Gate.
            </Text>

            <Alert
              color="blue"
              variant="light"
              icon={<IconInfoCircle size={16} />}
            >
              Импорт перезапишет файлы воркшопа для этой задачи.
            </Alert>

            <Group align="flex-end" wrap="wrap">
              <FileInput
                label="Архив пакета"
                placeholder="Выберите .zip файл"
                accept=".zip,application/zip"
                value={packageFile}
                onChange={setPackageFile}
                style={{minWidth: 320, flex: 1}}
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
    </Box>
  );
};
