import {Alert, Box, Button, FileInput, Group, Stack, Text} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconInfoCircle, IconUpload} from "@tabler/icons-react";
import {useQueryClient} from "@tanstack/react-query";
import {useState, useTransition} from "react";

import {SectionPaper} from "@/components/workshop/SectionPaper";
import {api, unwrap} from "@/lib/api";

import {draftKey} from "./draft/useDraft";

import type {ProblemDraft} from "@/contracts/core/v1/schema";
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
      // Import replaces the whole draft, so it is guarded by the revision the author last saw.
      const draft: ProblemDraft =
        queryClient.getQueryData<ProblemDraft>(draftKey(problemId)) ??
        (await queryClient.fetchQuery<ProblemDraft>({
          queryKey: draftKey(problemId),
          queryFn: () => unwrap(api.GET("/problems/{id}/draft", {params: {path: {id: problemId}}})),
        }));
      const body = new FormData();
      body.append("package", packageFile);
      const {data, error} = await api.POST("/problems/{id}/import", {
        params: {
          path: {id: problemId},
          query: {expected_revision: draft.revision},
          header: {"Idempotency-Key": crypto.randomUUID()},
        },
        body: body as unknown as {package: string},
      });
      if (error || !data) {
        notifications.show({
          title: "Ошибка импорта",
          message: error?.message ?? "Не удалось импортировать пакет",
          color: "red",
        });
        return;
      }

      notifications.show({
        title: "Пакет импортирован",
        message: "Черновик заменён содержимым архива",
        color: "green",
      });
      setPackageFile(null);
      queryClient.setQueryData(draftKey(problemId), data);
      void queryClient.invalidateQueries({queryKey: ["problem", problemId]});
    });
  };

  return (
    <Box p="lg">
      <Stack gap="lg" maw={900} mx="auto">
        <SectionPaper title="Импорт пакета">
          <Stack gap="sm">
            <Text size="sm" c="dimmed">
              Загрузите zip-архив задачи: он заменит текущий черновик.
              Поддерживаются форматы ICPC и Polygon, а также native-пакеты Gate.
            </Text>

            <Alert
              color="blue"
              variant="light"
              icon={<IconInfoCircle size={16} />}
            >
              Импорт заменит весь черновик этой задачи. Ранее собранные пакеты не изменятся.
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
