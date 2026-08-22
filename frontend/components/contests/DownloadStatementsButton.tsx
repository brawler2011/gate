"use client";

import {Button, Menu} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconChevronDown, IconFileTypePdf} from "@tabler/icons-react";
import {useState, type ReactNode} from "react";

import {env} from "@/lib/env";

interface DownloadStatementsButtonProps {
  orgLogin: string;
  contestLogin: string;
  variant?: "filled" | "light" | "outline" | "default";
  size?: "xs" | "sm" | "md" | "lg";
  fullWidth?: boolean;
}

export const DownloadStatementsButton = ({
  orgLogin,
  contestLogin,
  variant = "light",
  size = "sm",
  fullWidth = false,
}: DownloadStatementsButtonProps): ReactNode => {
  const [downloading, setDownloading] = useState(false);

  const handleDownload = async (lang: string = "ru") => {
    setDownloading(true);
    try {
      const backendUrl = env.getBackendApiUrl();
      const encodedOrg = encodeURIComponent(orgLogin);
      const encodedContest = encodeURIComponent(contestLogin);
      const encodedLang = encodeURIComponent(lang);

      const url = `${backendUrl}/organizations/${encodedOrg}/contests/${encodedContest}/statements.pdf?lang=${encodedLang}`;
      const response = await fetch(url, {
        method: "GET",
        credentials: "include",
      });

      if (!response.ok) {
        let message = "Не удалось сгенерировать PDF буклет задач";
        try {
          const errData = await response.json();
          if (errData?.message) {
            message = errData.message;
          }
        } catch {
          // ignore non-JSON error bodies
        }
        notifications.show({
          title: "Ошибка скачивания",
          message,
          color: "red",
        });
        return;
      }

      const blob = await response.blob();
      const objectUrl = window.URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = objectUrl;
      a.download = `${contestLogin}-statements-${lang}.pdf`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      window.URL.revokeObjectURL(objectUrl);

      notifications.show({
        title: "Успешно",
        message: "PDF буклет задач скачан",
        color: "green",
      });
    } catch (err) {
      console.error("PDF download failed:", err);
      notifications.show({
        title: "Ошибка",
        message: "Произошла ошибка при загрузке PDF",
        color: "red",
      });
    } finally {
      setDownloading(false);
    }
  };

  return (
    <Menu position="bottom-end" shadow="md">
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
        <Menu.Item onClick={() => handleDownload("ru")}>
          На русском языке (RU)
        </Menu.Item>
        <Menu.Item onClick={() => handleDownload("en")}>
          На английском языке (EN)
        </Menu.Item>
      </Menu.Dropdown>
    </Menu>
  );
};
