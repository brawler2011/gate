"use server";

import type {
    FileEntry,
    ListSubmissionsResponseModel,
    UpdateProblemLimitsRequest,
    UpdateProblemStatementRequest
} from '@contracts/core/v1';
import { Call, CallPublic, type ApiError } from './api';

export async function getContests(page: number = 1, pageSize: number = 10, search?: string, organizationId?: string) {
    return Call((client) => client.default.listWorkshopContests({page, pageSize, search, organizationId}));
}

export async function getUserContests(userId: string, page: number = 1, pageSize: number = 10, search?: string) {
    return Call((client) => client.default.listUserContests({id: userId, page, pageSize, search}));
}

export async function getProblems(page: number = 1, pageSize: number = 10, search?: string, order?: number, owner?: boolean, organizationId?: string, isTemplate?: boolean) {
    const params: {
        page: number;
        pageSize: number;
        search?: string;
        order?: number;
        owner?: boolean;
        organizationId?: string;
        isTemplate?: boolean;
    } = {
        page,
        pageSize,
        search,
        order,
        owner,
        organizationId,
        isTemplate,
    };

    return Call((client) => client.default.listProblems(params));
}

export async function getSubmissions(params: {
    page?: number;
    pageSize?: number;
    contestId?: string;
    userId?: string;
    problemId?: string;
    state?: number;
    sortOrder?: "asc" | "desc";
    language?: number;
}): Promise<[ApiError | null, ListSubmissionsResponseModel | null]> {
    // contestId is required for listContestSubmissions
    if (!params.contestId) {
        return [null, {submissions: [], pagination: {page: 1, total: 0}} as ListSubmissionsResponseModel];
    }
    
    const contestId = params.contestId;
    
    return Call((client) => client.default.listContestSubmissions({
        page: params.page ?? 1,
        pageSize: params.pageSize ?? 10,
        contestId: contestId,
        userId: params.userId,
        problemId: params.problemId,
        state: params.state,
        sortOrder: params.sortOrder ?? "desc",
        language: params.language,
    }));
}

/**
 * Get only current user's submissions in a contest
 * Requires ActionListOwnSubmissions permission on backend
 */
export async function getMySubmissions(params: {
    userId: string;
    contestId: string;
    page?: number;
    pageSize?: number;
    problemId?: string;
    state?: number;
    sortOrder?: "asc" | "desc";
    language?: number;
}) {
    return Call((client) => client.default.listContestSubmissions({
        page: params.page ?? 1,
        pageSize: params.pageSize ?? 10,
        contestId: params.contestId,
        userId: params.userId,
        problemId: params.problemId,
        state: params.state,
        sortOrder: params.sortOrder ?? "desc",
        language: params.language,
    }));
}

export async function listUsers(page: number = 1, pageSize: number = 10, search?: string, role?: string) {
    return Call((client) => client.default.listUsers({page, pageSize, search, role}));
}

export async function getUser(userId: string) {
    return Call((client) => client.default.getUser({id: userId}));
}


export async function getContest(contestId: string) {
    return Call((client) => client.default.getContest({contestId}));
}

export async function getContestProblem(problemId: string, contestId: string) {
    return Call((client) => client.default.getContestProblem({problemId, contestId}));
}

export async function getContestMembers(contestId: string, page: number = 1, pageSize: number = 10) {
    return Call((client) => client.default.listContestMembers({contestId, page, pageSize}));
}

export async function getProblem(problemId: string) {
    return Call((client) => client.default.getProblem({id: problemId}));
}

export async function getSubmission(submissionId: string) {
    return Call((client) => client.default.getSubmission({submissionId}));
}

export async function createContest(title: string, organizationId?: string) {
    return Call((client) => client.default.createContest({title, organizationId}));
}

export async function createProblem(title: string, organizationId?: string, templateId?: string) {
    return Call((client) => client.default.createProblem({title, organizationId, templateId}));
}

export async function updateProblem(
    problemId: string,
    data: {
        title?: string;
        legend?: string;
        input_format?: string;
        output_format?: string;
        notes?: string;
        scoring?: string;
        memory_limit?: number;
        time_limit?: number;
        visibility?: string;
        is_template?: boolean;
    }
) {
    return Call((client) => client.default.updateProblem({id: problemId, requestBody: data}));
}

export async function uploadProblemTests(id: string, file: File) {
    // FIXME: Bring back this functions to contract
    //return Call((client) => client.default.uploadProblemTests({id, formData: {file: file}}));
    return null;
}

export async function updateContest(
    contestId: string,
    data: {
        title?: string;
        description?: string;
        visibility?: string;
        monitor_scope?: string;
        submissions_list_scope?: string;
        submissions_review_scope?: string;
        start_time?: string | null;
        end_time?: string | null;
    }
) {
    return Call((client) => client.default.updateContest({contestId, requestBody: data}));
}

export async function addContestProblem(contestId: string, problemId: string) {
    return Call((client) => client.default.createContestProblem({contestId, problemId}));
}

export async function removeContestProblem(contestId: string, problemId: string) {
    return Call((client) => client.default.deleteContestProblem({problemId, contestId}));
}

export async function addContestMember(contestId: string, userId: string) {
    return Call((client) => client.default.createContestMember({contestId, userId}));
}

export async function removeContestMember(contestId: string, userId: string) {
    return Call((client) => client.default.deleteContestMember({userId, contestId}));
}

export async function searchProblems(title: string, owner?: boolean) {
    const params: {
        page: number;
        pageSize: number;
        search?: string;
        owner?: boolean;
    } = {
        page: 1,
        pageSize: 10,
        owner: owner
    };

    if (title && title.trim() !== "") {
        params.search = title.trim();
    }

    return Call((client) => client.default.listProblems(params));
}

export async function searchUsers(search: string) {
    return Call((client) => client.default.listUsers({
        page: 1,
        pageSize: 10,
        search: search,
    }));
}

export async function createSolution(
    problemId: string,
    contestId: string,
    language: number,
    submission: FormData
) {
    const solutionData = submission.get("submission");
    let solutionContent: string;
    
    if (solutionData instanceof File) {
        solutionContent = await solutionData.text();
    } else if (typeof solutionData === "string") {
        solutionContent = solutionData;
    } else {
        return [{ status: 400, message: "Invalid solution data type" }, null] as const;
    }
    
    return Call((client) => client.default.createSubmission({
        problemId,
        contestId,
        language,
        requestBody: {
            submission: solutionContent,
        },
    }));
}

export async function updateContestMemberRole(
  contestId: string,
  userId: string,
  newRole: string
) {
  return Call((client) => client.default.updateContestMember({ 
    contestId, 
    userId, 
    role: newRole 
  }));
}

export async function listAdminContests(
  page: number = 1,
  pageSize: number = 10,
  search?: string,
  sortBy: 'created_at' | 'updated_at' | 'title' = 'created_at',
  sortOrder: 'asc' | 'desc' = 'desc'
) {
  return Call((client) => client.default.listAdminContests({ page, pageSize, search, sortBy, sortOrder }));
}

export async function deleteContest(contestId: string) {
  return Call((client) => client.default.deleteContest({ contestId }));
}

// Blog API actions
export async function listPosts(
  page: number = 1,
  pageSize: number = 10,
  sortOrder: 'asc' | 'desc' = 'desc'
) {
  return Call((client) => client.default.listPosts({ page, pageSize, sortOrder }));
}

export async function listPostsPublic(
  page: number = 1,
  pageSize: number = 10,
  sortOrder: 'asc' | 'desc' = 'desc'
) {
  return CallPublic((client) => client.default.listPosts({ page, pageSize, sortOrder }));
}

export async function getPostById(id: string) {
  return Call((client) => client.default.getPostById({ id }));
}

export async function getPostByIdPublic(id: string) {
  return CallPublic((client) => client.default.getPostById({ id }));
}

export async function createPost(formData: {
  title?: string;
  description?: string;
  text?: string;
  preview_image?: Blob;
}) {
  return Call((client) => client.default.createPost({ formData }));
}

export async function patchPost(
  id: string,
  formData: {
    title?: string;
    description?: string;
    text?: string;
    preview_image?: Blob;
  }
) {
  return Call((client) => client.default.patchPostById({ id, formData }));
}

export async function deletePost(id: string) {
  return Call((client) => client.default.deletePostById({ id }));
}

// ─── Organizations ────────────────────────────────────────────────────────────

export async function listOrganizations(page: number = 1, pageSize: number = 20, search?: string) {
  return Call((client) => client.default.listOrganizations({ page, pageSize, search }));
}

export async function createOrganization(name: string) {
  return Call((client) => client.default.createOrganization({ name }));
}

export async function getOrganization(id: string) {
  return Call((client) => client.default.getOrganization({ id }));
}

export async function updateOrganization(id: string, data: { name?: string; description?: string }) {
  return Call((client) => client.default.updateOrganization({ id, requestBody: data }));
}

export async function deleteOrganization(id: string) {
  return Call((client) => client.default.deleteOrganization({ id }));
}

export async function listOrganizationMembers(orgId: string, page: number = 1, pageSize: number = 50) {
  return Call((client) => client.default.listOrganizationMembers({ id: orgId, page, pageSize }));
}

export async function addOrganizationMember(orgId: string, userId: string, role: 'owner' | 'admin' | 'member') {
  return Call((client) => client.default.addOrganizationMember({ id: orgId, userId, role }));
}

export async function removeOrganizationMember(orgId: string, userId: string) {
  return Call((client) => client.default.removeOrganizationMember({ id: orgId, userId }));
}

// ─── Teams ────────────────────────────────────────────────────────────────────

export async function listTeams(organizationId?: string, page: number = 1, pageSize: number = 50, search?: string) {
  return Call((client) => client.default.listTeams({ page, pageSize, search, organizationId }));
}

export async function createTeam(organizationId: string, name: string) {
  return Call((client) => client.default.createTeam({ requestBody: { name, organization_id: organizationId } }));
}

export async function getTeam(id: string) {
  return Call((client) => client.default.getTeam({ id }));
}

export async function updateTeam(id: string, data: { name?: string; description?: string }) {
  return Call((client) => client.default.updateTeam({ id, requestBody: data }));
}

export async function deleteTeam(id: string) {
  return Call((client) => client.default.deleteTeam({ id }));
}

export async function listTeamMembers(teamId: string, page: number = 1, pageSize: number = 50) {
  return Call((client) => client.default.listTeamMembers({ id: teamId, page, pageSize }));
}

export async function addTeamMember(teamId: string, userId: string) {
  return Call((client) => client.default.addTeamMember({ id: teamId, userId }));
}

export async function removeTeamMember(teamId: string, userId: string) {
  return Call((client) => client.default.removeTeamMember({ id: teamId, userId }));
}

// ─── Workshop Files ───────────────────────────────────────────────────────────


const badWorkshopRequest = (message: string): [ApiError, null] => [{ status: 400, message }, null];
type WorkshopFilesResponse = Promise<[ApiError | null, { files?: FileEntry[] } | null]>;
type WorkshopFileResponse = Promise<[ApiError | null, string | null]>;

const toText = async (data: Blob | string | ArrayBuffer | ArrayBufferView | null) => {
  if (data == null) return "";

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

export async function getWorkshopProblemLimits(problemId: string) {
  return Call((client) => client.default.getProblemLimits({ problemId }));
}

export async function updateWorkshopProblemLimits(problemId: string, requestBody: UpdateProblemLimitsRequest) {
  return Call((client) => client.default.updateProblemLimits({ problemId, requestBody }));
}

export async function getWorkshopProblemStatement(problemId: string, lang?: string) {
  return Call((client) => client.default.getProblemStatement({ problemId, lang }));
}

export async function updateWorkshopProblemStatement(problemId: string, requestBody: UpdateProblemStatementRequest, lang?: string) {
  return Call((client) => client.default.updateProblemStatement({ problemId, requestBody, lang }));
}


export async function listWorkshopCheckerFiles(problemId: string): WorkshopFilesResponse {
  return Call((client) => client.default.listProblemCheckers({ problemId }));
}

export async function getWorkshopCheckerFile(problemId: string, name: string): WorkshopFileResponse {
  const [error, data] = await Call((client) => client.default.getProblemChecker({ problemId, name }));
  if (error || !data) return [error, null];
  return [null, await toText(data)];
}

export async function createWorkshopCheckerFile(problemId: string, name: string, content: string) {
  const blob = new Blob([content], { type: 'application/octet-stream' });
  return Call((client) => client.default.createProblemChecker({ problemId, name, requestBody: blob }));
}

export async function updateWorkshopCheckerFile(problemId: string, name: string, content: string) {
  const blob = new Blob([content], { type: 'application/octet-stream' });
  return Call((client) => client.default.updateProblemChecker({ problemId, name, requestBody: blob }));
}

export async function listWorkshopGeneratorFiles(problemId: string): WorkshopFilesResponse {
  return Call((client) => client.default.listProblemGenerators({ problemId }));
}

export async function getWorkshopGeneratorFile(problemId: string, name: string): WorkshopFileResponse {
  const [error, data] = await Call((client) => client.default.getProblemGenerator({ problemId, name }));
  if (error || !data) return [error, null];
  return [null, await toText(data)];
}

export async function createWorkshopGeneratorFile(problemId: string, name: string, content: string) {
  const blob = new Blob([content], { type: 'application/octet-stream' });
  return Call((client) => client.default.createProblemGenerator({ problemId, name, requestBody: blob }));
}

export async function updateWorkshopGeneratorFile(problemId: string, name: string, content: string) {
  const blob = new Blob([content], { type: 'application/octet-stream' });
  return Call((client) => client.default.updateProblemGenerator({ problemId, name, requestBody: blob }));
}

export async function listWorkshopInteractorFiles(problemId: string): WorkshopFilesResponse {
  return Call((client) => client.default.listProblemInteractors({ problemId }));
}

export async function getWorkshopInteractorFile(problemId: string, name: string): WorkshopFileResponse {
  const [error, data] = await Call((client) => client.default.getProblemInteractor({ problemId, name }));
  if (error || !data) return [error, null];
  return [null, await toText(data)];
}

export async function createWorkshopInteractorFile(problemId: string, name: string, content: string) {
  const blob = new Blob([content], { type: 'application/octet-stream' });
  return Call((client) => client.default.createProblemInteractor({ problemId, name, requestBody: blob }));
}

export async function updateWorkshopInteractorFile(problemId: string, name: string, content: string) {
  const blob = new Blob([content], { type: 'application/octet-stream' });
  return Call((client) => client.default.updateProblemInteractor({ problemId, name, requestBody: blob }));
}

export async function listWorkshopMediaFiles(problemId: string): WorkshopFilesResponse {
  return Call((client) => client.default.listProblemMediaFiles({ problemId }));
}

export async function getWorkshopMediaFile(problemId: string, name: string): WorkshopFileResponse {
  const [error, data] = await Call((client) => client.default.getProblemMediaFile({ problemId, name }));
  if (error || !data) return [error, null];
  return [null, await toText(data)];
}

export async function createWorkshopMediaFile(problemId: string, name: string, content: string) {
  const blob = new Blob([content], { type: 'application/octet-stream' });
  return Call((client) => client.default.createProblemMediaFile({ problemId, name, requestBody: blob }));
}

export async function updateWorkshopMediaFile(problemId: string, name: string, content: string) {
  const blob = new Blob([content], { type: 'application/octet-stream' });
  return Call((client) => client.default.updateProblemMediaFile({ problemId, name, requestBody: blob }));
}

export async function listWorkshopSolutionFiles(problemId: string): WorkshopFilesResponse {
  return Call((client) => client.default.listProblemWorkshopSubmissions({ problemId }));
}

export async function getWorkshopSolutionFile(problemId: string, name: string): WorkshopFileResponse {
  const [error, data] = await Call((client) => client.default.getProblemWorkshopSubmission({ problemId, name }));
  if (error || !data) return [error, null];
  return [null, await toText(data)];
}

export async function createWorkshopSolutionFile(problemId: string, name: string, content: string) {
  const blob = new Blob([content], { type: 'application/octet-stream' });
  return Call((client) => client.default.createProblemWorkshopSubmission({ problemId, name, requestBody: blob }));
}

export async function updateWorkshopSolutionFile(problemId: string, name: string, content: string) {
  const blob = new Blob([content], { type: 'application/octet-stream' });
  return Call((client) => client.default.updateProblemWorkshopSubmission({ problemId, name, requestBody: blob }));
}

export async function listWorkshopTestFiles(problemId: string): WorkshopFilesResponse {
  return Call((client) => client.default.listProblemTests({ problemId }));
}

export async function getWorkshopTestFile(problemId: string, name: string): WorkshopFileResponse {
  const [error, data] = await Call((client) => client.default.getProblemTestFile({ problemId, name }));
  if (error || !data) return [error, null];
  return [null, await toText(data)];
}

export async function createWorkshopTestFile(problemId: string, name: string, content: string) {
  if (name === 'tests.json') {
    return badWorkshopRequest('tests/tests.json is reserved for tests configuration updates');
  }

  const blob = new Blob([content], { type: 'application/octet-stream' });
  return Call((client) => client.default.createProblemTestFile({ problemId, name, requestBody: blob }));
}

export async function updateWorkshopTestFile(problemId: string, name: string, content: string) {
  if (name === 'tests.json') {
    let testsConfig: Record<string, unknown>;
    try {
      testsConfig = JSON.parse(content) as Record<string, unknown>;
    } catch {
      return badWorkshopRequest('tests/tests.json must contain valid JSON');
    }
    return Call((client) => client.default.updateProblemTestsConfig({ problemId, requestBody: testsConfig }));
  }

  const blob = new Blob([content], { type: 'application/octet-stream' });
  return Call((client) => client.default.updateProblemTestFile({ problemId, name, requestBody: blob }));
}

export async function listWorkshopValidatorFiles(problemId: string): WorkshopFilesResponse {
  return Call((client) => client.default.listProblemValidators({ problemId }));
}

export async function getWorkshopValidatorFile(problemId: string, name: string): WorkshopFileResponse {
  const [error, data] = await Call((client) => client.default.getProblemValidator({ problemId, name }));
  if (error || !data) return [error, null];
  return [null, await toText(data)];
}

export async function createWorkshopValidatorFile(problemId: string, name: string, content: string) {
  const blob = new Blob([content], { type: 'application/octet-stream' });
  return Call((client) => client.default.createProblemValidator({ problemId, name, requestBody: blob }));
}

export async function updateWorkshopValidatorFile(problemId: string, name: string, content: string) {
  const blob = new Blob([content], { type: 'application/octet-stream' });
  return Call((client) => client.default.updateProblemValidator({ problemId, name, requestBody: blob }));
}

export async function publishProblem(problemId: string) {
  return Call((client) => client.default.publishProblem({ id: problemId }));
}

export async function importProblemPackage(problemId: string, packageFile: Blob) {
  return Call((client) =>
    client.default.importProblem({
      id: problemId,
      formData: {
        package: packageFile,
      },
    })
  );
}

export async function listProblemPackages(problemId: string) {
  return Call((client) => client.default.listProblemPackages({ id: problemId }));
}

export async function setWorkshopCheckerMain(problemId: string, name: string) {
  return Call((client) => client.default.setProblemCheckerMain({ problemId, requestBody: { name } }));
}

export async function setWorkshopGeneratorMain(problemId: string, name: string) {
  return Call((client) => client.default.setProblemGeneratorMain({ problemId, requestBody: { name } }));
}

export async function setWorkshopInteractorMain(problemId: string, name: string) {
  return Call((client) => client.default.setProblemInteractorMain({ problemId, requestBody: { name } }));
}

export async function setWorkshopValidatorMain(problemId: string, name: string) {
  return Call((client) => client.default.setProblemValidatorMain({ problemId, requestBody: { name } }));
}

export async function deleteProblem(id: string) {
  return Call((client) => client.default.deleteProblem({ id }));
}

export async function listSubmissions(params: {
  page: number;
  pageSize: number;
  contestId?: string;
  userId?: string;
  problemId?: string;
  state?: number;
  sortOrder?: "asc" | "desc";
  language?: number;
}): Promise<[ApiError | null, ListSubmissionsResponseModel | null]> {
  return Call((client) => client.default.listSubmissions(params));
}

