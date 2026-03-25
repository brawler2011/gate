"use client";

import { Stack } from "@mantine/core";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useMemo } from "react";
import type { FileEntry } from "@contracts/gateway/v1";
import { WorkshopHotbar, GENERAL_TAB, PACKAGES_TAB, STATEMENT_TAB } from "./WorkshopHotbar";
import { WorkshopFolderTab } from "./WorkshopFolderTab";
import { WorkshopGeneralTab } from "./WorkshopGeneralTab";
import { WorkshopPackagesTab } from "./WorkshopPackagesTab";
import { WorkshopStatementTab } from "./WorkshopStatementTab";

type Props = {
  problemId: string;
  initialFiles: FileEntry[];
};

export function WorkshopEditor({ problemId, initialFiles }: Props) {
  const router = useRouter();
  const searchParams = useSearchParams();

  const activeTab = searchParams.get("tab") ?? GENERAL_TAB;
  const selectedFile = searchParams.get("file");

  // Group files by top-level folder, excluding root files that go to general
  const folderMap = useMemo(() => {
    const map = new Map<string, FileEntry[]>();
    for (const file of initialFiles) {
      const path = file.path ?? "";
      if (!path.includes("/")) {
        // Top-level directory → register the tab (files inside will be added below)
        if (file.is_directory && path !== STATEMENT_TAB) {
          if (!map.has(path)) map.set(path, []);
        }
        // Root non-directory files (.gitignore, README.md, manifest.json) → general tab
        continue;
      }
      // File/dir inside a folder
      const folder = path.split("/")[0];
      if (folder === STATEMENT_TAB) continue;
      if (!map.has(folder)) map.set(folder, []);
      map.get(folder)!.push(file);
    }
    return map;
  }, [initialFiles]);

  const folders = useMemo(() => [...folderMap.keys()].sort(), [folderMap]);

  const setTab = useCallback(
    (tab: string) => {
      const params = new URLSearchParams(searchParams.toString());
      params.set("tab", tab);
      params.delete("file");
      router.push(`?${params.toString()}`, { scroll: false });
    },
    [router, searchParams]
  );

  const setFile = useCallback(
    (filePath: string) => {
      const params = new URLSearchParams(searchParams.toString());
      params.set("file", filePath);
      router.push(`?${params.toString()}`, { scroll: false });
    },
    [router, searchParams]
  );

  const handleFileCreated = useCallback(
    (filePath: string) => {
      // Refresh server data to pick up the new file, then select it
      router.refresh();
      const params = new URLSearchParams(searchParams.toString());
      params.set("file", filePath);
      router.push(`?${params.toString()}`, { scroll: false });
    },
    [router, searchParams]
  );

  return (
    <Stack gap={0} style={{ height: "calc(100vh - 70px)" }}>
      <WorkshopHotbar folders={folders} activeTab={activeTab} onTabChange={setTab} />

      {/* Tab content */}
      <Stack gap={0} style={{ flex: 1, overflow: "hidden", display: "flex" }}>
        {activeTab === GENERAL_TAB ? (
          <WorkshopGeneralTab problemId={problemId} />
        ) : activeTab === STATEMENT_TAB ? (
          <WorkshopStatementTab problemId={problemId} />
        ) : activeTab === PACKAGES_TAB ? (
          <WorkshopPackagesTab problemId={problemId} />
        ) : (
          <WorkshopFolderTab
            key={activeTab}
            problemId={problemId}
            folderName={activeTab}
            selectedFile={selectedFile}
            onFileSelect={setFile}
            onFileCreated={handleFileCreated}
          />
        )}
      </Stack>
    </Stack>
  );
}
