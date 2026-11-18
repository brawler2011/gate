import {useQuery} from "@tanstack/react-query";
import {useMemo} from "react";

import {api, unwrap} from "@/lib/api";

import type {SupportedLanguage} from "@/contracts/core/v1/schema";

export type {SupportedLanguage};

const NONE: SupportedLanguage[] = [];

/** The language catalog of the judge, in display order. */
export const useLanguages = (): SupportedLanguage[] => {
  const {data} = useQuery({
    queryKey: ["languages"],
    queryFn: async () => (await unwrap(api.GET("/languages"))).languages,
    staleTime: Infinity,
  });
  return data ?? NONE;
};

/** The configurations a contest allows; an empty list allows the whole catalog. */
const allowedLanguages = (catalog: SupportedLanguage[], allowed: string[] | undefined): SupportedLanguage[] =>
  allowed?.length ? catalog.filter((language) => allowed.includes(language.id)) : catalog;

/** The catalog configurations contestants of a contest may submit in. */
export const useContestLanguages = (allowed: string[] | undefined): SupportedLanguage[] => {
  const catalog = useLanguages();
  return useMemo(() => allowedLanguages(catalog, allowed), [catalog, allowed]);
};

/** The display name of a configuration, or its identifier when the catalog lacks it. */
export const languageName = (catalog: SupportedLanguage[], id: string | undefined): string =>
  catalog.find((language) => language.id === id)?.name ?? id ?? "";

/** Prefer the saved matching configuration, then the first match in catalog order. */
export const languageForExtension = (
  catalog: SupportedLanguage[], extension: string, preferred?: string,
): SupportedLanguage | undefined => {
  const matches = catalog.filter((language) => language.extensions.includes(extension.toLowerCase()));
  return matches.find((language) => language.id === preferred) ?? matches[0];
};
