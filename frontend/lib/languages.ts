import {useQuery} from "@tanstack/react-query";
import {useMemo} from "react";

import {api, unwrap} from "@/lib/api";

import type {SupportedLanguage} from "@/contracts/core/v1/schema";

export type {SupportedLanguage};

const NONE: SupportedLanguage[] = [];

export const useLanguages = (): SupportedLanguage[] => {
  const {data} = useQuery({
    queryKey: ["languages"],
    queryFn: async () => (await unwrap(api.GET("/languages"))).languages,
    staleTime: Infinity,
  });
  return data ?? NONE;
};

const allowedLanguages = (catalog: SupportedLanguage[], allowed: string[] | undefined): SupportedLanguage[] =>
  allowed?.length ? catalog.filter((language) => allowed.includes(language.id)) : catalog;

export const useContestLanguages = (allowed: string[] | undefined): SupportedLanguage[] => {
  const catalog = useLanguages();
  return useMemo(() => allowedLanguages(catalog, allowed), [catalog, allowed]);
};

// Output-only submissions carry this language; they run nothing.
export const OUTPUTS_LANGUAGE = "output";

export const languageName = (catalog: SupportedLanguage[], id: string | undefined): string =>
  id === OUTPUTS_LANGUAGE ? "Ответы" : catalog.find((language) => language.id === id)?.name ?? id ?? "";

export const languageForExtension = (
  catalog: SupportedLanguage[], extension: string, preferred?: string,
): SupportedLanguage | undefined => {
  const matches = catalog.filter((language) => language.extensions.includes(extension.toLowerCase()));
  return matches.find((language) => language.id === preferred) ?? matches[0];
};
