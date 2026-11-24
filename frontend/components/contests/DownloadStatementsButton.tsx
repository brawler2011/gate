import {Alert, Button, Code, Menu, Stack} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconChevronDown, IconFileTypePdf} from "@tabler/icons-react";
import {useState, type ReactNode} from "react";

import {api} from "@/lib/api";
import {problemMessage} from "@/lib/problemText";

import type {ContestBooklet} from "@/contracts/core/v1/schema";

interface DownloadStatementsButtonProps {
  contestId: string;
  contestLogin: string;
  variant?: "filled" | "light" | "outline" | "default";
  size?: "xs" | "sm" | "md" | "lg";
  fullWidth?: boolean;
}

const LANGUAGE_NAMES: Record<string, string> = {ru: "На русском языке", en: "На английском языке"};
const STATE_NOTES: Record<ContestBooklet["state"], string> = {building: " (собирается)", ready: "", failed: " (ошибка сборки)"};

const useBooklets = (contestId: string) =>
  api.useQuery("get", "/contests/{contest_id}/booklets", {params: {path: {contest_id: contestId}}}, {
    refetchInterval: (query) => (query.state.data?.booklets.some((b) => b.state === "building") ? 3000 : false),
  });

export const DownloadStatementsButton = ({
  contestId,
  contestLogin,
  variant = "light",
  size = "sm",
  fullWidth = false,
}: DownloadStatementsButtonProps): ReactNode => {
  const [downloading, setDownloading] = useState(false);
  const query = useBooklets(contestId);
  const booklets = query.data?.booklets ?? [];

  const handleDownload = async (lang: string) => {
    setDownloading(true);
    try {
      const {data, error} = await api.GET("/contests/{contest_id}/statements.pdf", {
        params: {path: {contest_id: contestId}, query: {lang}},
        parseAs: "blob",
      });
      if (error) {
        notifications.show({title: "Ошибка скачивания", message: problemMessage(error), color: "red"});
        void query.refetch();
        return;
      }
      const objectUrl = window.URL.createObjectURL(data);
      const a = document.createElement("a");
      a.href = objectUrl;
      a.download = `${contestLogin}-statements-${lang}.pdf`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      window.URL.revokeObjectURL(objectUrl);
    } finally {
      setDownloading(false);
    }
  };

  return (
    <Menu position="bottom-end" shadow="md" onOpen={() => void query.refetch()}>
      <Menu.Target>
        <Button
          leftSection={<IconFileTypePdf size={16} />}
          rightSection={<IconChevronDown size={14} />}
          variant={variant}
          size={size}
          loading={downloading}
          fullWidth={fullWidth}
        >
          Скачать условия (PDF)
        </Button>
      </Menu.Target>

      <Menu.Dropdown>
        {booklets.length === 0 && <Menu.Item disabled>Условий пока нет</Menu.Item>}
        {booklets.map((booklet) => (
          <Menu.Item key={booklet.language} disabled={booklet.state !== "ready"} onClick={() => handleDownload(booklet.language)}>
            {LANGUAGE_NAMES[booklet.language] ?? booklet.language.toUpperCase()}
            {STATE_NOTES[booklet.state]}
          </Menu.Item>
        ))}
      </Menu.Dropdown>
    </Menu>
  );
};

// Only contest managers receive diagnostics, so others see nothing here.
export const BookletDiagnostics = ({contestId}: {contestId: string}): ReactNode => {
  const failed = (useBooklets(contestId).data?.booklets ?? []).filter((b) => b.state === "failed" && b.diagnostic);
  if (failed.length === 0) {
    return null;
  }
  return (
    <Stack gap="xs">
      {failed.map((booklet) => (
        <Alert key={booklet.language} color="red" title={`Буклет (${booklet.language}) не собрался`}>
          <Code block>{booklet.diagnostic}</Code>
        </Alert>
      ))}
    </Stack>
  );
};
