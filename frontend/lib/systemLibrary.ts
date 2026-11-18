import {useQuery} from "@tanstack/react-query";

import {api, unwrap} from "@/lib/api";

import type {SystemChecker, SystemLibrary, SystemLibraryFile} from "@/contracts/core/v1/schema";

export type {SystemChecker, SystemLibraryFile};

const useLibrary = <T,>(select: (library: SystemLibrary) => T): T | undefined =>
  useQuery({
    queryKey: ["system-library"],
    queryFn: async () => unwrap(api.GET("/lib")),
    staleTime: Infinity,
    select,
  }).data;

const files = (library: SystemLibrary) => library.files;
const checkers = (library: SystemLibrary) => library.checkers;

export const useSystemLibrary = (): SystemLibraryFile[] | undefined => useLibrary(files);

export const useStandardCheckers = (): SystemChecker[] | undefined => useLibrary(checkers);
