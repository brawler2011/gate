"use server";

import { Call } from './api';

export async function getContests(page: number = 1, pageSize: number = 10, search?: string) {
    return Call((client) => client.default.listWorkshopContests({page, pageSize, search}));
}

export async function getPublicContests(page: number = 1, pageSize: number = 10, search?: string) {
    return Call((client) => client.default.listPublicContests({page, pageSize, search}));
}

export async function getUserContests(userId: string, page: number = 1, pageSize: number = 10, search?: string) {
    return Call((client) => client.default.listUserContests({id: userId, page, pageSize, search}));
}

export async function getProblems(page: number = 1, pageSize: number = 10, search?: string, order?: number, owner?: boolean) {
    const params: {
        page: number;
        pageSize: number;
        search?: string;
        order?: number;
        owner?: boolean;
    } = {
        page,
        pageSize,
        search,
        order,
        owner,
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
}) {
    // contestId is required for listContestSubmissions
    if (!params.contestId) {
        return [null, {submissions: [], pagination: {page: 1, total: 0}}] as const;
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

export async function createContest(title: string) {
    return Call((client) => client.default.createContest({title}));
}

export async function createProblem(title: string) {
    return Call((client) => client.default.createProblem({title}));
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
    return Call((client) => client.default.uploadProblemTests({id, formData: {file: file}}));
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
        title?: string;
        owner?: boolean;
    } = {
        page: 1,
        pageSize: 10,
        owner: owner
    };

    if (title && title.trim() !== "") {
        params.title = title.trim();
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
