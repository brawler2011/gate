"use client";

import {
  Alert,
  Box,
  Button,
  Center,
  Group,
  Loader,
  Paper,
  Stack,
  Text,
  ThemeIcon,
  Title,
} from "@mantine/core";
import {IconAlertCircle, IconCheck} from "@tabler/icons-react";
import Image from "next/image";
import Link from "next/link";
import {useSearchParams} from "next/navigation";
import {Suspense, useEffect, useState, type ReactNode} from "react";

import {api} from "@/lib/api";

const ConfirmEmailChangePage = (): ReactNode => {
  return (
    <Suspense
      fallback={
        <Center h="100vh">
          <Loader size="lg" />
        </Center>
      }
    >
      <ConfirmEmailChangeContent />
    </Suspense>
  );
};

const ConfirmEmailChangeContent = () => {
  const searchParams = useSearchParams();
  const token = searchParams.get("token") || "";

  const [loading, setLoading] = useState(true);
  const [success, setSuccess] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!token) {
      setError("Токен подтверждения отсутствует в ссылке.");
      setLoading(false);
      return;
    }

    let isMounted = true;

    const confirm = async () => {
      try {
        const [err] = await api.confirmEmailChange({
          requestBody: {token},
        });

        if (!isMounted) return;

        if (!err) {
          setSuccess(true);
        } else {
          setError(err?.message || "Недействительная или устаревшая ссылка подтверждения смены email.");
        }
      } catch {
        if (isMounted) {
          setError("Не удалось подключиться к серверу");
        }
      } finally {
        if (isMounted) {
          setLoading(false);
        }
      }
    };

    confirm();

    return () => {
      isMounted = false;
    };
  }, [token]);

  return (
    <Box
      style={{
        minHeight: "100vh",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        padding: "1rem",
      }}
    >
      <Stack
        align="center"
        gap={32}
        style={{width: "min(550px, calc(100vw - 2rem))"}}
      >
        <Link href="/" style={{textDecoration: "none", color: "inherit"}}>
          <Group justify="center" gap="md">
            <Image
              src="/gate_logo.svg"
              alt="Gate"
              width={56}
              height={56}
              priority
            />
            <Title order={1} fz={36}>
              Gate
            </Title>
          </Group>
        </Link>

        <Paper
          radius="md"
          p={32}
          withBorder
          shadow="sm"
          style={{width: "100%"}}
        >
          {loading ? (
            <Stack align="center" gap="md" py="xl">
              <Loader size="lg" />
              <Text fz={15} c="dimmed">
                Подтверждаем новый адрес электронной почты...
              </Text>
            </Stack>
          ) : success ? (
            <Stack align="center" gap="md" ta="center">
              <ThemeIcon size={64} radius="xl" color="green" variant="light">
                <IconCheck size={36} />
              </ThemeIcon>

              <Title order={2} fz={22}>
                Email успешно обновлён!
              </Title>

              <Text c="dimmed" fz={15}>
                Ваш новый адрес электронной почты активирован и привязан к вашей учетной записи.
              </Text>

              <Button
                component={Link}
                href="/settings"
                variant="filled"
                size="sm"
                mt="md"
              >
                Перейти в настройки
              </Button>
            </Stack>
          ) : (
            <Stack align="center" gap="md" ta="center">
              <ThemeIcon size={64} radius="xl" color="red" variant="light">
                <IconAlertCircle size={36} />
              </ThemeIcon>

              <Title order={2} fz={22}>
                Не удалось подтвердить смену email
              </Title>

              {error && (
                <Alert
                  icon={<IconAlertCircle size={18} />}
                  color="red"
                  title="Ошибка"
                  radius="md"
                  w="100%"
                  ta="left"
                >
                  {error}
                </Alert>
              )}

              <Button
                component={Link}
                href="/settings"
                variant="outline"
                size="sm"
                mt="md"
              >
                Вернуться в настройки
              </Button>
            </Stack>
          )}
        </Paper>
      </Stack>
    </Box>
  );
};

export default ConfirmEmailChangePage;
