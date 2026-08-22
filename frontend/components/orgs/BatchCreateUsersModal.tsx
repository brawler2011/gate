"use client";

import {
  ActionIcon,
  Alert,
  Badge,
  Button,
  CopyButton,
  Group,
  Modal,
  NumberInput,
  ScrollArea,
  Stack,
  Table,
  Text,
  TextInput,
  Tooltip,
} from "@mantine/core";
import {useForm} from "@mantine/form";
import {notifications} from "@mantine/notifications";
import {
  IconCheck,
  IconCopy,
  IconDownload,
  IconInfoCircle,
  IconPrinter,
  IconUserPlus,
} from "@tabler/icons-react";
import {useState} from "react";

import {api} from "@/lib/api";

import type {BatchCreatedUserItem} from "@/contracts/core/v1";
import type {ReactNode} from "react";

type Props = {
  opened: boolean;
  onClose: () => void;
  orgLogin: string;
  onSuccess?: () => void;
};

export const BatchCreateUsersModal = ({
  opened,
  onClose,
  orgLogin,
  onSuccess,
}: Props): ReactNode => {
  const [loading, setLoading] = useState(false);
  const [createdUsers, setCreatedUsers] = useState<BatchCreatedUserItem[] | null>(null);

  const form = useForm({
    initialValues: {
      prefix: "olymp_",
      count: 20,
      ttlDays: 30 as number | undefined,
    },
    validate: {
      prefix: (value) => {
        if (!value.trim()) {
          return "Укажите префикс логина";
        }
        if (value.length > 20) {
          return "Префикс не должен превышать 20 символов";
        }
        if (!/^[a-zA-Z0-9_-]+$/.test(value)) {
          return "Префикс может содержать только латинские буквы, цифры, дефис и подчеркивание";
        }
        return null;
      },
      count: (value) => {
        if (value < 1 || value > 500) {
          return "Количество должно быть от 1 до 500";
        }
        return null;
      },
    },
  });

  const handleClose = () => {
    if (createdUsers) {
      onSuccess?.();
    }
    setCreatedUsers(null);
    form.reset();
    onClose();
  };

  const handleSubmit = async (values: typeof form.values) => {
    setLoading(true);
    const [error, data] = await api.batchCreateOrganizationUsers({
      login: orgLogin,
      requestBody: {
        prefix: values.prefix.trim(),
        count: values.count,
        ttl_days: values.ttlDays && values.ttlDays > 0 ? values.ttlDays : undefined,
      },
    });
    setLoading(false);

    if (error) {
      notifications.show({
        title: "Ошибка создания пользователей",
        message: error.message,
        color: "red",
      });
      return;
    }

    if (data?.users) {
      setCreatedUsers(data.users);
      notifications.show({
        title: "Успех",
        message: `Создано ${data.users.length} пользователей`,
        color: "green",
      });
    }
  };

  const handleDownloadCsv = () => {
    if (!createdUsers || createdUsers.length === 0) {
      return;
    }

    const headers = ["Логин", "Пароль", "Срок действия"];
    const rows = createdUsers.map((u) => [
      u.username,
      u.password,
      u.expires_at ? new Date(u.expires_at).toLocaleDateString("ru-RU") : "Бессрочно",
    ]);

    const csvContent = [
      headers.join(","),
      ...rows.map((r) => r.map((cell) => `"${cell}"`).join(",")),
    ].join("\n");

    const blob = new Blob(["\uFEFF" + csvContent], {type: "text/csv;charset=utf-8;"});
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.setAttribute("href", url);
    link.setAttribute("download", `users-${orgLogin}-${new Date().toISOString().slice(0, 10)}.csv`);
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  };

  const handleCopyAll = () => {
    if (!createdUsers || createdUsers.length === 0) {
      return;
    }

    const text = createdUsers
      .map((u) => `${u.username}\t${u.password}\t${u.expires_at ? new Date(u.expires_at).toLocaleDateString("ru-RU") : "Бессрочно"}`)
      .join("\n");

    navigator.clipboard.writeText(text).then(() => {
      notifications.show({
        title: "Скопировано",
        message: `Данные ${createdUsers.length} пользователей скопированы в буфер обмена`,
        color: "blue",
      });
    });
  };

  const handlePrintCards = () => {
    if (!createdUsers || createdUsers.length === 0) {
      return;
    }

    const printWindow = window.open("", "_blank");
    if (!printWindow) {
      notifications.show({
        title: "Ошибка",
        message: "Не удалось открыть окно печати. Разрешите всплывающие окна.",
        color: "red",
      });
      return;
    }

    const cardsHtml = createdUsers
      .map((u) => {
        const expiryHtml = u.expires_at
          ? `<div class="row expiry"><span class="label">Действителен до:</span><span class="value">${new Date(u.expires_at).toLocaleDateString("ru-RU")}</span></div>`
          : "";
        return `
      <div class="card">
        <div class="card-header">
          <span class="org-name">${orgLogin}</span>
          <span class="badge">Временный доступ</span>
        </div>
        <div class="card-body">
          <div class="row">
            <span class="label">Логин:</span>
            <span class="value login">${u.username}</span>
          </div>
          <div class="row">
            <span class="label">Пароль:</span>
            <span class="value password">${u.password}</span>
          </div>
          ${expiryHtml}
        </div>
        <div class="card-footer">
          <span>Сайт: ${window.location.origin}</span>
        </div>
      </div>`;
      })
      .join("");

    const fullHtml = `
      <!DOCTYPE html>
      <html>
        <head>
          <title>Карточки участников - ${orgLogin}</title>
          <meta charset="utf-8" />
          <style>
            @page {
              size: A4;
              margin: 10mm;
            }
            body {
              font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
              margin: 0;
              padding: 0;
              color: #111;
            }
            .grid {
              display: grid;
              grid-template-columns: repeat(2, 1fr);
              gap: 8mm;
            }
            .card {
              border: 1.5px dashed #555;
              border-radius: 6px;
              padding: 12px 16px;
              box-sizing: border-box;
              page-break-inside: avoid;
              background: #fafafa;
            }
            .card-header {
              display: flex;
              justify-content: space-between;
              align-items: center;
              border-bottom: 1px solid #ddd;
              padding-bottom: 6px;
              margin-bottom: 10px;
            }
            .org-name {
              font-weight: 700;
              font-size: 15px;
            }
            .badge {
              font-size: 11px;
              background: #e2e8f0;
              padding: 2px 6px;
              border-radius: 4px;
              color: #475569;
            }
            .card-body {
              display: flex;
              flex-direction: column;
              gap: 6px;
            }
            .row {
              display: flex;
              justify-content: space-between;
              align-items: baseline;
            }
            .label {
              font-size: 13px;
              color: #666;
            }
            .value {
              font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
              font-weight: 600;
              font-size: 14px;
            }
            .login {
              font-size: 15px;
              color: #0f172a;
            }
            .password {
              color: #2563eb;
              letter-spacing: 0.5px;
            }
            .expiry {
              margin-top: 4px;
              font-size: 11px;
              color: #888;
            }
            .card-footer {
              margin-top: 8px;
              border-top: 1px dotted #ccc;
              padding-top: 4px;
              font-size: 11px;
              color: #777;
              text-align: right;
            }
          </style>
        </head>
        <body>
          <div class="grid">
            ${cardsHtml}
          </div>
          <script>
            window.onload = function() {
              window.print();
            };
          </script>
        </body>
      </html>
    `;

    printWindow.document.open();
    printWindow.document.write(fullHtml);
    printWindow.document.close();
  };

  return (
    <Modal
      opened={opened}
      onClose={handleClose}
      title={
        <Group gap="xs">
          <IconUserPlus size={20} />
          <Text fw={600} size="lg">
            {createdUsers ? "Учётные данные созданных пользователей" : "Пакетная генерация пользователей"}
          </Text>
        </Group>
      }
      size={createdUsers ? "xl" : "md"}
      centered
    >
      {!createdUsers ? (
        <form onSubmit={form.onSubmit(handleSubmit)}>
          <Stack gap="md">
            <Text size="sm" c="dimmed">
              Быстрая генерация временных аккаунтов для олимпиад или контестов с автоматическим добавлением в
              организацию.
            </Text>

            <TextInput
              label="Префикс логина"
              placeholder="olymp_"
              description="Имена пользователей будут иметь вид: prefix_01, prefix_02..."
              required
              {...form.getInputProps("prefix")}
            />

            <NumberInput
              label="Количество аккаунтов"
              placeholder="10"
              min={1}
              max={500}
              required
              {...form.getInputProps("count")}
            />

            <NumberInput
              label="Срок действия (в днях)"
              placeholder="Например, 30"
              description="После истечения срока вход в аккаунт будет заблокирован. Оставьте пустым для бессрочных."
              min={1}
              max={365}
              {...form.getInputProps("ttlDays")}
            />

            <Group justify="flex-end" mt="md">
              <Button variant="default" onClick={handleClose}>
                Отмена
              </Button>
              <Button type="submit" loading={loading} leftSection={<IconUserPlus size={16} />}>
                Сгенерировать
              </Button>
            </Group>
          </Stack>
        </form>
      ) : (
        <Stack gap="md">
          <Alert
            icon={<IconInfoCircle size={18} />}
            title="Обязательно сохраните данные"
            color="yellow"
          >
            Пользователи успешно созданы. Пароли отображаются только один раз и не сохраняются в открытом виде на
            сервере. Скопируйте или скачайте файл прямо сейчас.
          </Alert>

          <Group justify="space-between" align="center">
            <Badge size="lg" variant="light" color="blue">
              Создано: {createdUsers.length}
            </Badge>
            <Group gap="xs">
              <Button
                variant="default"
                size="xs"
                leftSection={<IconCopy size={14} />}
                onClick={handleCopyAll}
              >
                Скопировать всё
              </Button>
              <Button
                variant="default"
                size="xs"
                leftSection={<IconDownload size={14} />}
                onClick={handleDownloadCsv}
              >
                Скачать CSV
              </Button>
              <Button
                variant="default"
                size="xs"
                leftSection={<IconPrinter size={14} />}
                onClick={handlePrintCards}
              >
                Печать карточек
              </Button>
            </Group>
          </Group>

          <ScrollArea h={340} type="auto">
            <Table highlightOnHover withTableBorder>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>№</Table.Th>
                  <Table.Th>Логин</Table.Th>
                  <Table.Th>Пароль</Table.Th>
                  <Table.Th>Срок действия</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {createdUsers.map((u, idx) => (
                  <Table.Tr key={u.id}>
                    <Table.Td>{idx + 1}</Table.Td>
                    <Table.Td>
                      <Group gap="xs" wrap="nowrap">
                        <Text ff="monospace" fw={600} size="sm">
                          {u.username}
                        </Text>
                        <CopyButton value={u.username} timeout={2000}>
                          {({copied, copy}) => (
                            <Tooltip label={copied ? "Скопировано" : "Копировать логин"}>
                              <ActionIcon color={copied ? "teal" : "gray"} variant="subtle" size="xs" onClick={copy}>
                                {copied ? <IconCheck size={12} /> : <IconCopy size={12} />}
                              </ActionIcon>
                            </Tooltip>
                          )}
                        </CopyButton>
                      </Group>
                    </Table.Td>
                    <Table.Td>
                      <Group gap="xs" wrap="nowrap">
                        <Text ff="monospace" c="blue.6" fw={600} size="sm">
                          {u.password}
                        </Text>
                        <CopyButton value={u.password} timeout={2000}>
                          {({copied, copy}) => (
                            <Tooltip label={copied ? "Скопировано" : "Копировать пароль"}>
                              <ActionIcon color={copied ? "teal" : "gray"} variant="subtle" size="xs" onClick={copy}>
                                {copied ? <IconCheck size={12} /> : <IconCopy size={12} />}
                              </ActionIcon>
                            </Tooltip>
                          )}
                        </CopyButton>
                      </Group>
                    </Table.Td>
                    <Table.Td>
                      <Text size="xs" c="dimmed">
                        {u.expires_at
                          ? new Date(u.expires_at).toLocaleDateString("ru-RU")
                          : "Бессрочно"}
                      </Text>
                    </Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          </ScrollArea>

          <Group justify="flex-end" mt="sm">
            <Button onClick={handleClose}>Готово</Button>
          </Group>
        </Stack>
      )}
    </Modal>
  );
};
