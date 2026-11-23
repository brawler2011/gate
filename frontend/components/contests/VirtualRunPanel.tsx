import {Alert, Button, Group, Modal, Stack, Text} from "@mantine/core";
import {useDisclosure} from "@mantine/hooks";
import {notifications} from "@mantine/notifications";
import {IconPlayerPlay} from "@tabler/icons-react";
import {useQueryClient} from "@tanstack/react-query";
import {useEffect, useState, type ReactNode} from "react";

import {useVirtualRun} from "@/hooks/useVirtualRun";
import {api} from "@/lib/api";
import {problemMessage} from "@/lib/problemText";

import type {ContestModel} from "@/contracts/core/v1/schema";

const clock = (ms: number): string => {
  const total = Math.max(0, Math.floor(ms / 1000));
  const pad = (n: number) => n.toString().padStart(2, "0");
  return `${pad(Math.floor(total / 3600))}:${pad(Math.floor(total / 60) % 60)}:${pad(total % 60)}`;
};

const Remaining = ({until}: {until: string}): ReactNode => {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, []);
  return <Text span fw={700} ff="monospace">{clock(Date.parse(until) - now)}</Text>;
};

export const VirtualRunPanel = ({contest, orgLogin}: {contest: ContestModel; orgLogin: string}): ReactNode => {
  const {run, replaying} = useVirtualRun(orgLogin, contest.login);
  const client = useQueryClient();
  const [opened, {open, close}] = useDisclosure(false);
  const [busy, setBusy] = useState(false);
  const path = {org_login: orgLogin, contest_login: contest.login};

  const act = async (request: () => Promise<{error?: unknown}>, failure: string) => {
    setBusy(true);
    const {error} = await request();
    setBusy(false);
    close();
    if (error) {
      notifications.show({title: "Ошибка", message: problemMessage(error, failure), color: "red"});
      return;
    }
    await client.invalidateQueries();
  };

  if (run && replaying) {
    return (
      <Alert color="violet" variant="light" mb="md" title="Виртуальное участие">
        <Group justify="space-between" align="center">
          <Text size="sm">До конца осталось <Remaining until={run.ends_at} /></Text>
          <Button size="xs" variant="subtle" color="violet" loading={busy}
            onClick={() => act(() => api.POST("/organizations/{org_login}/contests/{contest_login}/virtual/finish", {params: {path}}),
              "Не удалось завершить виртуальное участие")}>
            Завершить досрочно
          </Button>
        </Group>
      </Alert>
    );
  }
  if (!contest.permissions.start_virtual || !contest.start_time || !contest.end_time) {
    return null;
  }
  const duration = clock(Date.parse(contest.end_time) - Date.parse(contest.start_time));
  return (
    <>
      <Alert color="violet" variant="light" mb="md" title="Виртуальное участие">
        <Group justify="space-between" align="center">
          <Text size="sm">Контест завершён. Его можно пройти виртуально с тем же временем: {duration}.</Text>
          <Button size="xs" color="violet" leftSection={<IconPlayerPlay size={14} />} onClick={open}>Начать</Button>
        </Group>
      </Alert>
      <Modal opened={opened} onClose={close} title="Начать виртуальное участие?" centered>
        <Stack gap="md">
          <Text size="sm">
            Таймер на {duration} запустится сразу. Монитор будет показывать положение участников
            на тот же момент от начала, а ваши посылки попадут только в виртуальную таблицу.
            Пройти контест виртуально можно один раз.
          </Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={close}>Отмена</Button>
            <Button color="violet" loading={busy}
              onClick={() => act(() => api.POST("/organizations/{org_login}/contests/{contest_login}/virtual", {params: {path}}),
                "Не удалось начать виртуальное участие")}>
              Начать
            </Button>
          </Group>
        </Stack>
      </Modal>
    </>
  );
};
