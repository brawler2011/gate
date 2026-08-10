import {api, type ApiError} from "./api";

const toText = async (data: Blob | string | ArrayBuffer | ArrayBufferView | null) => {
  if (data === null || data === undefined) {
    return "";
  }

  if (typeof data === 'string') {
    return data;
  }

  if (typeof (data as Blob).text === 'function') {
    return (data as Blob).text();
  }

  if (data instanceof ArrayBuffer) {
    return new TextDecoder().decode(data);
  }

  if (ArrayBuffer.isView(data)) {
    return new TextDecoder().decode(data);
  }

  return String(data);
};

export const getWorkshopCheckerFile = async (
  problemId: string,
  name: string
): Promise<[ApiError | null, string | null]> => {
  const [error, data] = await api.getProblemChecker({problemId, name});
  if (error || !data) {
    return [error, null];
  }
  return [null, await toText(data)];
};

export const createWorkshopCheckerFile = async (problemId: string, name: string, content: string) => {
  const blob = new Blob([content], {type: 'application/octet-stream'});
  return api.createProblemChecker({problemId, name, requestBody: blob});
};

export const updateWorkshopCheckerFile = async (problemId: string, name: string, content: string) => {
  const blob = new Blob([content], {type: 'application/octet-stream'});
  return api.updateProblemChecker({problemId, name, requestBody: blob});
};

export const setWorkshopCheckerMain = async (problemId: string, name: string) => {
  return api.setProblemCheckerMain({problemId, requestBody: {name}});
};

export const getWorkshopGeneratorFile = async (
  problemId: string,
  name: string
): Promise<[ApiError | null, string | null]> => {
  const [error, data] = await api.getProblemGenerator({problemId, name});
  if (error || !data) {
    return [error, null];
  }
  return [null, await toText(data)];
};

export const createWorkshopGeneratorFile = async (problemId: string, name: string, content: string) => {
  const blob = new Blob([content], {type: 'application/octet-stream'});
  return api.createProblemGenerator({problemId, name, requestBody: blob});
};

export const updateWorkshopGeneratorFile = async (problemId: string, name: string, content: string) => {
  const blob = new Blob([content], {type: 'application/octet-stream'});
  return api.updateProblemGenerator({problemId, name, requestBody: blob});
};

export const getWorkshopInteractorFile = async (
  problemId: string,
  name: string
): Promise<[ApiError | null, string | null]> => {
  const [error, data] = await api.getProblemInteractor({problemId, name});
  if (error || !data) {
    return [error, null];
  }
  return [null, await toText(data)];
};

export const createWorkshopInteractorFile = async (problemId: string, name: string, content: string) => {
  const blob = new Blob([content], {type: 'application/octet-stream'});
  return api.createProblemInteractor({problemId, name, requestBody: blob});
};

export const updateWorkshopInteractorFile = async (problemId: string, name: string, content: string) => {
  const blob = new Blob([content], {type: 'application/octet-stream'});
  return api.updateProblemInteractor({problemId, name, requestBody: blob});
};

export const setWorkshopInteractorMain = async (problemId: string, name: string) => {
  return api.setProblemInteractorMain({problemId, requestBody: {name}});
};

export const uploadWorkshopMediaBinary = async (formData: FormData) => {
  const problemId = formData.get("problemId") as string;
  const name = formData.get("name") as string;
  const file = formData.get("file") as File | null;
  if (!problemId || !name || !file) {
    return [{status: 400, message: "Файл не выбран"}, null] as const;
  }
  const arrayBuffer = await file.arrayBuffer();
  const blob = new Blob([arrayBuffer], {type: file.type || 'application/octet-stream'});
  return api.createProblemMediaFile({problemId, name, requestBody: blob});
};

export const getWorkshopSolutionFile = async (
  problemId: string,
  name: string
): Promise<[ApiError | null, string | null]> => {
  const [error, data] = await api.getProblemWorkshopSubmission({problemId, name});
  if (error || !data) {
    return [error, null];
  }
  return [null, await toText(data)];
};

export const createWorkshopSolutionFile = async (problemId: string, name: string, content: string) => {
  const blob = new Blob([content], {type: 'application/octet-stream'});
  return api.createProblemWorkshopSubmission({problemId, name, requestBody: blob});
};

export const updateWorkshopSolutionFile = async (problemId: string, name: string, content: string) => {
  const blob = new Blob([content], {type: 'application/octet-stream'});
  return api.updateProblemWorkshopSubmission({problemId, name, requestBody: blob});
};

export const getWorkshopTestFile = async (
  problemId: string,
  name: string
): Promise<[ApiError | null, string | null]> => {
  const [error, data] = await api.getProblemTestFile({problemId, name});
  if (error || !data) {
    return [error, null];
  }
  return [null, await toText(data)];
};

export const createWorkshopTestFile = async (problemId: string, name: string, content: string) => {
  if (name === 'tests.json') {
    return [{status: 400, message: 'tests/tests.json is reserved for tests configuration updates'}, null] as const;
  }

  const blob = new Blob([content], {type: 'application/octet-stream'});
  return api.createProblemTestFile({problemId, name, requestBody: blob});
};

export const updateWorkshopTestFile = async (problemId: string, name: string, content: string) => {
  if (name === 'tests.json') {
    let testsConfig: Record<string, unknown>;
    try {
      testsConfig = JSON.parse(content) as Record<string, unknown>;
    } catch {
      return [{status: 400, message: 'tests/tests.json must contain valid JSON'}, null] as const;
    }
    return api.updateProblemTestsConfig({problemId, requestBody: testsConfig});
  }

  const blob = new Blob([content], {type: 'application/octet-stream'});
  return api.updateProblemTestFile({problemId, name, requestBody: blob});
};

export const getWorkshopValidatorFile = async (
  problemId: string,
  name: string
): Promise<[ApiError | null, string | null]> => {
  const [error, data] = await api.getProblemValidator({problemId, name});
  if (error || !data) {
    return [error, null];
  }
  return [null, await toText(data)];
};

export const createWorkshopValidatorFile = async (problemId: string, name: string, content: string) => {
  const blob = new Blob([content], {type: 'application/octet-stream'});
  return api.createProblemValidator({problemId, name, requestBody: blob});
};

export const updateWorkshopValidatorFile = async (problemId: string, name: string, content: string) => {
  const blob = new Blob([content], {type: 'application/octet-stream'});
  return api.updateProblemValidator({problemId, name, requestBody: blob});
};

export const setWorkshopValidatorMain = async (problemId: string, name: string) => {
  return api.setProblemValidatorMain({problemId, requestBody: {name}});
};

export const generateWorkshopTests = async (
  problemId: string,
  generatorName: string,
  testNumbers: number[],
  args?: string[][]
) => {
  return api.generateTests({
    problemId,
    requestBody: {
      generator_name: generatorName,
      test_numbers: testNumbers,
      arguments: args,
    },
  });
};

export const testWorkshopSolution = async (problemId: string, solutionPath: string, testNumbers?: number[]) => {
  return api.testSolution({
    problemId,
    requestBody: {
      solution_path: solutionPath,
      test_numbers: testNumbers,
    },
  });
};

export const createSolution = async (
  problemId: string,
  contestId: string,
  language: number,
  submission: FormData
) => {
  const solutionData = submission.get("submission");
  let solutionContent: string;

  if (solutionData instanceof File) {
    solutionContent = await solutionData.text();
  } else if (typeof solutionData === "string") {
    solutionContent = solutionData;
  } else {
    return [{status: 400, message: "Invalid solution data type"}, null] as const;
  }

  return api.createSubmission({
    problemId,
    contestId,
    language,
    requestBody: {
      submission: solutionContent,
    },
  });
};
