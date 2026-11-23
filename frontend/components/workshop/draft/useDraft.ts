import {notifications} from "@mantine/notifications";
import {useMutation, useQuery, useQueryClient, type UseQueryResult} from "@tanstack/react-query";
import {useCallback} from "react";

import {api, unwrap, type Problem} from "@/lib/api";
import {fieldText} from "@/lib/problemText";

import {describePointer, type OpResult} from "./draftOps";

import type {
  ProblemDraft,
  ProblemDraftCheckerTest,
  ProblemDraftTest,
  ProblemDraftValidatorTest,
} from "@/contracts/core/v1/schema";

export const draftKey = (problemId: string): readonly ["problem-draft", string] => ["problem-draft", problemId];

type DraftError = Error & Partial<Problem>;

const isConflict = (error: unknown): boolean => (error as DraftError | undefined)?.status === 409;

const FIELD_LABELS: Record<string, string> = {
  title: "название",
  short_name: "короткое имя",
  type: "тип задачи",
  time_limit_ms: "ограничение времени",
  memory_limit_mb: "ограничение памяти",
  input_file: "входной файл",
  output_file: "выходной файл",
  max_score: "максимальный балл",
  name: "имя",
  language: "язык",
  source: "исходный код",
  standard: "стандартный чекер",
  content: "текст",
  auto_update: "автообновление",
  object_id: "содержимое",
  points: "баллы",
  ordinal: "номер",
  policy: "политика",
  depends_on: "зависимости",
  subtask_id: "подзадача",
  subtasks: "подзадачи",
  verdicts: "вердикты",
  verdict: "вердикт",
  group: "группа",
  input: "ввод",
  output: "вывод",
  answer: "ответ",
  mode: "режим",
  generator: "генератор",
  solution: "решение",
  args: "аргументы",
};

const GENERIC_ENTRY = "Черновик";

// Names the entry and, below it, the field: "Тест 1, генератор".
const describeField = (draft: ProblemDraft, pointer: string): string => {
  const parts = pointer.split("/").slice(1);
  const entry = describePointer(draft, pointer);
  const field = parts.toReversed().find((part) => !/^\d+$/.test(part));
  const label = field && parts[0] !== "statements" ? FIELD_LABELS[field] : undefined;
  if (!label || (field === parts[0] && entry !== GENERIC_ENTRY)) {
    return entry;
  }
  return entry === GENERIC_ENTRY ? label.charAt(0).toUpperCase() + label.slice(1) : `${entry}, ${label}`;
};

// Resolve error pointers against the draft sent with the request.
export const errorMessage = (error: unknown, draft?: ProblemDraft): string => {
  const fields = (error as DraftError | undefined)?.errors;
  if (draft && fields && fields.length > 0) {
    return fields.map((field) => {
      const other = field.params?.with;
      const text = typeof other === "string" ? `${fieldText(field)} (${describeField(draft, other)})` : fieldText(field);
      return `${describeField(draft, field.pointer)}: ${text}`;
    }).join("\n");
  }
  return error instanceof Error ? error.message : "Неизвестная ошибка";
};

export const useProblemDraft = (problemId: string): UseQueryResult<ProblemDraft, Error> =>
  useQuery({
    queryKey: draftKey(problemId),
    queryFn: () => unwrap(api.GET("/problems/{id}/draft", {params: {path: {id: problemId}}})),
    // Avoid background refreshes overwriting edits while the author types.
    staleTime: Infinity,
  });

const reportError = (title: string, error: unknown, draft?: ProblemDraft): void => {
  if (isConflict(error)) {
    notifications.show({
      title: "Черновик изменён в другом месте",
      message: "Загружена актуальная версия. Повторите изменение.",
      color: "red",
    });
    return;
  }
  notifications.show({title, message: errorMessage(error, draft), color: "red", style: {whiteSpace: "pre-line"}});
};

type DraftChange = (draft: ProblemDraft) => ProblemDraft | OpResult;

type SaveDraft = {save: (change: DraftChange) => Promise<boolean>; isSaving: boolean};

export const useSaveDraft = (problemId: string): SaveDraft => {
  const queryClient = useQueryClient();
  const mutation = useMutation({
    mutationFn: async (next: ProblemDraft): Promise<ProblemDraft> =>
      unwrap(
        api.PUT("/problems/{id}/draft", {
          params: {
            path: {id: problemId},
            query: {expected_revision: next.revision},
          },
          body: next,
        }),
      ),
    onSuccess: (saved) => {
      queryClient.setQueryData(draftKey(problemId), saved);
    },
    onError: (error, sent) => {
      reportError("Не удалось сохранить черновик", error, sent);
      if (isConflict(error)) {
        void queryClient.invalidateQueries({queryKey: draftKey(problemId)});
      }
    },
  });

  const {mutateAsync} = mutation;
  const save = useCallback(
    async (change: DraftChange): Promise<boolean> => {
      const current = queryClient.getQueryData<ProblemDraft>(draftKey(problemId));
      if (!current) {
        return false;
      }
      const result = change(current);
      const next = "draft" in result || "error" in result ? result : {draft: result};
      if ("error" in next) {
        notifications.show({title: "Изменение невозможно", message: next.error, color: "red"});
        return false;
      }
      try {
        await mutateAsync(next.draft);
        return true;
      } catch {
        return false;
      }
    },
    [mutateAsync, problemId, queryClient],
  );
  return {save, isSaving: mutation.isPending};
};

export type UploadSlot = "asset" | "input" | "output" | "answer";

// The change names one entry; files fill its slots and keep the entry's other object IDs.
export type UploadRequest = {
  change:
    | {asset_name: string}
    | {test: ProblemDraftTest}
    | {checker_test: ProblemDraftCheckerTest}
    | {validator: string; validator_test: ProblemDraftValidatorTest};
  files: Partial<Record<UploadSlot, Blob>>;
};

export const useUploadToDraft = (
  problemId: string,
): {upload: (request: UploadRequest) => Promise<ProblemDraft>; isUploading: boolean} => {
  const queryClient = useQueryClient();
  const mutation = useMutation({
    mutationFn: async (request: UploadRequest): Promise<ProblemDraft> => {
      const current = queryClient.getQueryData<ProblemDraft>(draftKey(problemId));
      if (!current) {
        throw new Error("Черновик ещё не загружен");
      }
      const body = new FormData();
      body.append("metadata", JSON.stringify(request.change));
      for (const [slot, file] of Object.entries(request.files)) {
        if (file) {
          body.append(slot, file);
        }
      }
      return unwrap(
        api.POST("/problems/{id}/draft/uploads", {
          params: {
            path: {id: problemId},
            query: {expected_revision: current.revision},
          },
          body: body as unknown as {metadata: string},
        }),
      );
    },
    onSuccess: (saved) => {
      queryClient.setQueryData(draftKey(problemId), saved);
    },
    onError: (error) => {
      reportError("Не удалось загрузить файл", error, queryClient.getQueryData<ProblemDraft>(draftKey(problemId)));
      if (isConflict(error)) {
        void queryClient.invalidateQueries({queryKey: draftKey(problemId)});
      }
    },
  });
  return {upload: mutation.mutateAsync, isUploading: mutation.isPending};
};

export const objectUrl = (problemId: string, objectId: string): string =>
  `/api/problems/${problemId}/draft/objects/${objectId}`;
