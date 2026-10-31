import {Container, Group, Paper, Skeleton, Stack} from "@mantine/core";

import type {ReactNode} from "react";

const Loading = (): ReactNode => (
  <Container size="md" py="lg">
    <Stack gap="xl" role="status" aria-label="Загрузка команды">
      <Skeleton height={20} width={180} />
      <Group wrap="nowrap">
        <Skeleton height={56} width={56} circle />
        <Stack style={{flex: 1}}>
          <Skeleton height={28} width="50%" />
          <Skeleton height={16} width="75%" />
        </Stack>
      </Group>
      <Paper radius="md" p="md" withBorder>
        <Stack>
          <Skeleton height={36} />
          <Skeleton height={40} />
          <Skeleton height={40} />
          <Skeleton height={40} />
        </Stack>
      </Paper>
    </Stack>
  </Container>
);

export default Loading;
