import {Button, Group, Modal, Stack, Text, Textarea} from "@mantine/core";
import {useState} from "react";

import type {ReactNode} from "react";

type Props = {
  path: string;
  content: string;
  disabled?: boolean;
  saving?: boolean;
  onSave: (content: string) => void;
  onDelete: () => void;
};

/** Plain-text editor for one draft file with save and confirmed delete. */
export const SourceEditor = ({path, content, disabled, saving, onSave, onDelete}: Props): ReactNode => {
  const [text, setText] = useState(content);
  const [confirming, setConfirming] = useState(false);
  return (
    <Stack gap="sm">
      <Text ff="monospace" size="sm" fw={600}>
        {path}
      </Text>
      <Textarea
        value={text}
        onChange={(e) => setText(e.currentTarget.value)}
        autosize
        minRows={16}
        maxRows={36}
        spellCheck={false}
        styles={{input: {fontFamily: "var(--mantine-font-family-monospace)", fontSize: "var(--mantine-font-size-sm)", whiteSpace: "pre"}}}
      />
      <Group justify="space-between">
        <Button variant="subtle" color="red" disabled={disabled} onClick={() => setConfirming(true)}>
          Удалить
        </Button>
        <Button disabled={text === content || disabled} loading={saving} onClick={() => onSave(text)}>
          Сохранить
        </Button>
      </Group>
      <Modal opened={confirming} onClose={() => setConfirming(false)} title="Удалить файл?" centered>
        <Stack>
          <Text size="sm">Файл {path} будет удалён из черновика.</Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setConfirming(false)}>
              Отмена
            </Button>
            <Button
              color="red"
              onClick={() => {
                setConfirming(false);
                onDelete();
              }}
            >
              Удалить
            </Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  );
};
