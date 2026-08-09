"use client";

import { ActionIcon, Box, Flex, Group, Paper, Stack, Text, Title, Tooltip } from "@mantine/core";
import { useClipboard } from "@mantine/hooks";
import { IconCheck, IconCopy } from "@tabler/icons-react";
import katex from "katex";
import "katex/dist/katex.min.css";
import { useEffect, useRef } from "react";
import ReactMarkdown from "react-markdown";
import rehypeKatex from "rehype-katex";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import "./Problem.css";

type Props = {
  problem: {
    id?: string;
    problem_id?: string;
    title: string;
    time_limit: number;
    memory_limit: number;
    legend_html: string;
    input_format_html: string;
    output_format_html: string;
    notes_html: string;
    scoring_html: string;
    created_at: string;
    updated_at: string;
    samples?: Array<{
      input: string;
      output: string;
    }>;
  };

  problemId?: string;
  letter?: string;
};

const prettifyTimeLimit = (time_limit: number) => {
  if (time_limit % 1000 === 0) {
    return `${time_limit / 1000} сек`;
  }

  return `${time_limit} мс`;
};

const prettifyMemoryLimit = (memory_limit: number) => {
  if (memory_limit % 1000 === 0) {
    return `${memory_limit / 1000} ГБ`;
  }

  return `${memory_limit} МБ`;
};

const CopyableSection = ({ label, value }: { label: string; value: string }) => {
  const clipboard = useClipboard({ timeout: 2000 });
  return (
    <Stack gap="xs" style={{ flex: "1 1 200px", minWidth: 0 }}>
      <Group justify="space-between" align="center" h={28}>
        <Text fw={600} size="sm">{label}</Text>
        <Tooltip label={clipboard.copied ? "Скопировано!" : "Скопировать"} position="top" withArrow>
          <ActionIcon
            variant="subtle"
            color={clipboard.copied ? "green" : "gray"}
            onClick={() => clipboard.copy(value)}
            size="sm"
          >
            {clipboard.copied ? <IconCheck size={16} /> : <IconCopy size={16} />}
          </ActionIcon>
        </Tooltip>
      </Group>
      <Box
        component="pre"
        p="xs"
        bg="light-dark(var(--mantine-color-gray-0), var(--mantine-color-dark-6))"
        style={{
          border: "1px solid light-dark(var(--mantine-color-gray-3), var(--mantine-color-dark-5))",
          borderRadius: "var(--mantine-radius-sm)",
          overflowX: "auto",
          fontFamily: "var(--mantine-font-monospace)",
          fontSize: "var(--mantine-font-size-sm)",
          whiteSpace: "pre",
          margin: 0,
        }}
      >
        {value}
      </Box>
    </Stack>
  );
};

const renderSafeImage = (problemId?: string) => {
  const SafeImage = ({
    node: _node,
    src,
    alt,
    ...props
  }: React.ImgHTMLAttributes<HTMLImageElement> & { node?: unknown }) => {
    if (!src) {
      return null;
    }

    let width: string | undefined;
    let height: string | undefined;
    let isCentered = false;
    let cleanSrc = src;

    const hashIndex = src.indexOf("#");
    if (hashIndex !== -1) {
      cleanSrc = src.slice(0, hashIndex);
      const hashParts = src.slice(hashIndex + 1).split("#");

      for (const part of hashParts) {
        if (part === "center" || part === "middle") {
          isCentered = true;
        } else if (/^\d+(x\d*)?$/.test(part)) {
          if (part.includes("x")) {
            const [w, h] = part.split("x");
            if (w) {
              width = `${w}px`;
            }
            if (h) {
              height = `${h}px`;
            }
          } else {
            width = `${part}px`;
          }
        } else if (/^(\d+(x\d*)?)-center$/.test(part) || /^center-(\d+(x\d*)?)$/.test(part)) {
          isCentered = true;
          const num = part.replace(/-?center-?/, "");
          if (num.includes("x")) {
            const [w, h] = num.split("x");
            if (w) {
              width = `${w}px`;
            }
            if (h) {
              height = `${h}px`;
            }
          } else {
            width = `${num}px`;
          }
        }
      }
    }

    if (
      problemId &&
      !cleanSrc.startsWith("http://") &&
      !cleanSrc.startsWith("https://") &&
      !cleanSrc.startsWith("data:") &&
      !cleanSrc.startsWith("/api/")
    ) {
      const filename = cleanSrc.replace(/^\.\//, "").replace(/^media\//, "");
      cleanSrc = `/api/problems/${problemId}/media/${filename}`;
    }

    return (
      <img
        src={cleanSrc}
        alt={alt || ""}
        style={{
          maxWidth: "100%",
          width: width,
          height: height || "auto",
          display: isCentered ? "block" : undefined,
          margin: isCentered ? "0 auto" : undefined,
        }}
        {...props}
      />
    );
  };
  return SafeImage;
};

const StatementContent = ({ value, problemId }: { value: string; problemId?: string }) => {
  return (
    <div className="content">
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkMath]}
        rehypePlugins={[rehypeKatex]}
        components={{
          img: renderSafeImage(problemId),
        }}
      >
        {value}
      </ReactMarkdown>
    </div>
  );
};

const Problem = ({ problem, letter, problemId }: Props) => {
  letter = letter || "A";
  const activeProblemId = problemId || problem.id || problem.problem_id;

  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (ref.current) {
      const mathElements = ref.current.querySelectorAll(".math");
      mathElements.forEach((element) => {
        if (
          element instanceof HTMLElement &&
          !element.hasAttribute("data-rendered")
        ) {
          katex.render(element.textContent || "", element, {
            throwOnError: false,
            displayMode: element.classList.contains("display"),
          });

          // mark as rendered
          element.setAttribute("data-rendered", "true");
        }
      });
    }

    return () => {
      if (ref.current) {
        ref.current.querySelectorAll(".math").forEach((element) => {
          element.removeAttribute("data-rendered");
        });
      }
    };
  }, [problem, letter]);

  return (
    <Stack className="container" ref={ref} gap="md">
      <Stack align="center" gap={0} w="fit-content" mx="auto" mb="sm">
        <Title order={2}>
          {letter}. {problem.title}
        </Title>
        <Stack align="center" gap={0}>
          <Text>
            ограничение по времени: {prettifyTimeLimit(problem.time_limit)}
          </Text>
          <Text>
            ограничение по памяти: {prettifyMemoryLimit(problem.memory_limit)}
          </Text>
        </Stack>
      </Stack>
      {problem.legend_html && <StatementContent value={problem.legend_html} problemId={activeProblemId} />}
      {problem.input_format_html && (
        <Stack gap="xs">
          <Title order={3}>Входные данные</Title>
          <StatementContent value={problem.input_format_html} problemId={activeProblemId} />
        </Stack>
      )}
      {problem.output_format_html && (
        <Stack gap="xs">
          <Title order={3}>Выходные данные</Title>
          <StatementContent value={problem.output_format_html} problemId={activeProblemId} />
        </Stack>
      )}
      {problem.samples && problem.samples.length > 0 && (
        <Stack gap="xs">
          <Title order={3}>Примеры</Title>
          <Stack gap="md">
            {problem.samples.map((sample, index) => (
              <Paper
                key={index}
                withBorder
                p="md"
                radius="md"
                bg="light-dark(var(--mantine-color-white), var(--mantine-color-dark-7))"
              >
                <Stack gap="xs">
                  <Text fw={700} size="sm" c="dimmed">
                    Пример {index + 1}
                  </Text>
                  <Flex gap="md" wrap="wrap">
                    <CopyableSection label="Входные данные" value={sample.input} />
                    <CopyableSection label="Выходные данные" value={sample.output} />
                  </Flex>
                </Stack>
              </Paper>
            ))}
          </Stack>
        </Stack>
      )}
      {problem.notes_html && (
        <Stack gap="xs">
          <Title order={3}>Примечание</Title>
          <StatementContent value={problem.notes_html} problemId={activeProblemId} />
        </Stack>
      )}
    </Stack>
  );
};

export { Problem };
