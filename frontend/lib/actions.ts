"use server";

import { Call, type ApiError } from './api';
import type {
  ListSubmissionsResponseModel,
  ListOrganizationsResponseModel,
  GetOrganizationResponseModel,
  ListOrganizationMembersResponseModel,
  ListTeamsResponseModel,
  GetTeamResponseModel,
  ListTeamMembersResponseModel,
} from '@contracts/gateway/v1';

export async function getContests(page: number = 1, pageSize: number = 10, search?: string, organizationId?: string) {
    return Call((client) => client.default.listWorkshopContests({page, pageSize, search, organizationId}));
}

export async function getPublicContests(page: number = 1, pageSize: number = 10, search?: string) {
    return Call((client) => client.default.listPublicContests({page, pageSize, search}));
}

export async function getUserContests(userId: string, page: number = 1, pageSize: number = 10, search?: string) {
    return Call((client) => client.default.listUserContests({id: userId, page, pageSize, search}));
}

export async function getProblems(page: number = 1, pageSize: number = 10, search?: string, order?: number, owner?: boolean, organizationId?: string) {
    const params: {
        page: number;
        pageSize: number;
        search?: string;
        order?: number;
        owner?: boolean;
        organizationId?: string;
    } = {
        page,
        pageSize,
        search,
        order,
        owner,
        organizationId,
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

export async function createProblem(title: string, organizationId?: string) {
    return Call((client) => client.default.createProblem({title, organizationId}));
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
        is_private?: boolean;
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

export async function getPostById(id: string) {
  return Call((client) => client.default.getPostById({ id }));
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

export async function initProblemWorkshop(problemId: string) {
  return Call((client) => client.default.initProblemWorkshop({ problemId }));
}

export async function listWorkshopFiles(problemId: string, path?: string) {
  return Call((client) => client.default.listWorkshopFiles({ problemId, path }));
}

export async function getWorkshopFile(problemId: string, path: string) {
  return Call((client) => client.default.getWorkshopFile({ problemId, path }));
}

export async function saveWorkshopFile(problemId: string, path: string, content: string) {
  const blob = new Blob([content], { type: 'application/octet-stream' });
  return Call((client) => client.default.updateWorkshopFile({ problemId, path, requestBody: blob }));
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
