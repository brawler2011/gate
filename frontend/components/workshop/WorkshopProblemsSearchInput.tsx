"use client";

import { TextInput } from "@mantine/core";
import { IconSearch } from "@tabler/icons-react";

type Props = {
  value: string;
  onChange: (value: string) => void;
};

export function WorkshopProblemsSearchInput({ value, onChange }: Props) {
  return (
    <TextInput
      placeholder="Поиск задач..."
      leftSection={<IconSearch size={16} />}
      value={value}
      onChange={(e) => onChange(e.currentTarget.value)}
      radius="md"
      size="md"
    />
  );
}

