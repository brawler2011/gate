export type WorkshopFileTabProps = {
  problemId: string;
  selectedFile: string | null;
  onFileSelect: (filePath: string) => void;
  onFileCreated: (filePath: string) => void;
};
