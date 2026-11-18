import {useQuery} from "@tanstack/react-query";

import {api, unwrap} from "@/lib/api";

import type {SystemLibraryFile} from "@/contracts/core/v1/schema";

export type {SystemLibraryFile};

/** The system library files that auto-updated draft library files build with; undefined until loaded. */
export const useSystemLibrary = (): SystemLibraryFile[] | undefined =>
  useQuery({
    queryKey: ["system-library"],
    queryFn: async () => (await unwrap(api.GET("/lib"))).files,
    staleTime: Infinity,
  }).data;
