import {notifications} from "@mantine/notifications";
import {useMutation, useQuery, useQueryClient, type UseQueryResult} from "@tanstack/react-query";
import {useCallback} from "react";

import {api, unwrap} from "@/lib/api";

import type {OpResult} from "./draftOps";
import type {ProblemDraft, ProblemDraftTest} from "@/contracts/core/v1/schema";

export const draftKey = (problemId: string): readonly ["problem-draft", string] => ["problem-draft", problemId];

type DraftConflictError = Error & {status?: number};

const isConflict = (error: unknown): boolean => (error as DraftConflictError | undefined)?.status === 409;

export const useProblemDraft = (problemId: string): UseQueryResult<ProblemDraft, Error> =>
  useQuery({
    queryKey: draftKey(problemId),
    queryFn: () => unwrap(api.GET("/problems/{id}/draft", {params: {path: {id: problemId}}})),
    // The draft is only replaced by a mutation result or an explicit reload,
    // never silently under an author who is typing.
    staleTime: Infinity,
  });

const reportError = (title: string, error: unknown): void => {
  if (isConflict(error)) {
    notifications.show({
      title: "Черновик изменён в другом месте",
      message: "Загружена актуальная версия. Повторите изменение.",
      color: "red",
    });
    return;
  }
  notifications.show({title, message: error instanceof Error ? error.message : "Неизвестная ошибка", color: "red"});
};

/**
 * Applies `change` to the latest known draft and stores it with optimistic
 * locking on its revision. Every call is a new user action with its own
 * idempotency key.
 */
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
            header: {"Idempotency-Key": crypto.randomUUID()},
          },
          body: next,
        }),
      ),
    onSuccess: (saved) => {
      queryClient.setQueryData(draftKey(problemId), saved);
    },
    onError: (error) => {
      reportError("Не удалось сохранить черновик", error);
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

export type UploadRequest =
  | {part: "asset"; assetPath: string; file: Blob}
  | {part: "input" | "answer"; test: ProblemDraftTest; file: Blob}
  | {part: "pair"; test: ProblemDraftTest; file: Blob; answer: Blob};

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
      const metadata =
        request.part === "asset"
          ? {part: request.part, asset_path: request.assetPath}
          : {part: request.part, test: request.test};
      const body = new FormData();
      body.append("metadata", JSON.stringify(metadata));
      body.append("file", request.file);
      if (request.part === "pair") {
        body.append("answer", request.answer);
      }
      return unwrap(
        api.POST("/problems/{id}/draft/uploads", {
          params: {
            path: {id: problemId},
            query: {expected_revision: current.revision},
            header: {"Idempotency-Key": crypto.randomUUID()},
          },
          body: body as unknown as {metadata: string; file: string; answer?: string},
        }),
      );
    },
    onSuccess: (saved) => {
      queryClient.setQueryData(draftKey(problemId), saved);
    },
    onError: (error) => {
      reportError("Не удалось загрузить файл", error);
      if (isConflict(error)) {
        void queryClient.invalidateQueries({queryKey: draftKey(problemId)});
      }
    },
  });
  return {upload: mutation.mutateAsync, isUploading: mutation.isPending};
};

export const objectUrl = (problemId: string, objectId: string): string =>
  `/api/problems/${problemId}/draft/objects/${objectId}`;
