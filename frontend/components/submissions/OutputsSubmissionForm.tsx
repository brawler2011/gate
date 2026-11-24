import {Button, FileInput, Stack, Text} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconFiles} from "@tabler/icons-react";
import {useState, useTransition, type ReactNode} from "react";

import {api} from "@/lib/api";
import {problemMessage} from "@/lib/problemText";
import {APP_COLORS} from "@/lib/theme/colors";

type Props = {
  contestId: string;
  problemId: string;
  disabled?: boolean;
  onSubmitted?: () => void;
};

export const OutputsSubmissionForm = ({contestId, problemId, disabled = false, onSubmitted}: Props): ReactNode => {
  const [files, setFiles] = useState<File[]>([]);
  const [isPending, startTransition] = useTransition();

  const submit = () => {
    startTransition(async () => {
      const body = new FormData();
      for (const file of files) {
        body.append("files", file);
      }
      const {error} = await api.POST("/submissions/outputs", {
        params: {query: {problem_id: problemId, contest_id: contestId}},
        body: body as unknown as {files: string[]},
      });
      if (error) {
        notifications.show({title: "Ответы не отправлены", message: problemMessage(error, "Не удалось отправить ответы"), color: "red"});
        return;
      }
      setFiles([]);
      onSubmitted?.();
    });
  };

  return (
    <Stack gap="sm">
      <FileInput
        multiple
        clearable
        label="Выходные файлы"
        placeholder="Файлы или .zip"
        leftSection={<IconFiles size={16} />}
        value={files}
        onChange={setFiles}
        disabled={disabled}
      />
      <Text size="xs" c="dimmed">
        Имя файла заканчивается номером теста: 01.out, output_1.txt. Тесты без файла получат WA.
      </Text>
      <Button
        onClick={submit}
        loading={isPending}
        disabled={disabled || files.length === 0}
        color={APP_COLORS.submissions}
      >
        Отправить ответы
      </Button>
    </Stack>
  );
};
