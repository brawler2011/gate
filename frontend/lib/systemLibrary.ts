import {useQuery} from "@tanstack/react-query";

import {api, unwrap} from "@/lib/api";

import type {SystemLibraryFile} from "@/contracts/core/v1/schema";

export type {SystemLibraryFile};

const NONE: SystemLibraryFile[] = [];

/** The system library files that auto-updated draft library files build with. */
export const useSystemLibrary = (): SystemLibraryFile[] => {
  const {data} = useQuery({
    queryKey: ["system-library"],
    queryFn: async () => (await unwrap(api.GET("/lib"))).files,
    staleTime: Infinity,
  });
  return data ?? NONE;
};
