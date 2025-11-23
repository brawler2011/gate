"use server";

import { ListContestMembersResponseModel, ListSubmissionsResponseModel, ListUsersResponseModel, SubmissionsListItemModel } from '../../contracts/core/v1';
import {Call} from './api';

export async function getContests(page: number = 1, pageSize: number = 10, search?: string) {
    try {
        const response = await Call((client) => client.default.listWorkshopContests({page, pageSize, search}));
        return response;
    } catch (error) {
        console.error('Failed to fetch contests:', error);
        return null;
    }
}

export async function getPublicContests(page: number = 1, pageSize: number = 10, search?: string) {
    try {
        const response = await Call((client) => client.default.listPublicContests({page, pageSize, search}));
        return response;
    } catch (error) {
        console.error('Failed to fetch public contests:', error);
        return null;
    }
}

export async function getUserContests(userId: string, page: number = 1, pageSize: number = 10, search?: string) {
    try {
        const response = await Call((client) => client.default.listUserContests({id: userId, page, pageSize, search}));
        return response;
    } catch (error) {
        console.error('Failed to fetch user contests:', error);
        return null;
    }
}

export async function getProblems(page: number = 1, pageSize: number = 10, search?: string, order?: number, owner?: boolean) {
    try {
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

        const response = await Call((client) => client.default.listProblems(params));
        return response;
    } catch (error) {
        console.error('Failed to fetch problems:', error);
        return null;
    }
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
}): Promise<ListSubmissionsResponseModel> {
    try {
        // contestId is required for listContestSubmissions
        if (!params.contestId) {
            return {submissions: [], pagination: {page: 1, total: 0}};
        }
        
        const contestId = params.contestId; // TypeScript now knows this is a string
        
        const response = await Call((client) => client.default.listContestSubmissions({
            page: params.page ?? 1,
            pageSize: params.pageSize ?? 10,
            contestId: contestId,
            userId: params.userId,
            problemId: params.problemId,
            state: params.state,
            sortOrder: params.sortOrder ?? "desc",
            language: params.language,
        }));
        return response;
    } catch (error: any) {
        const status = error?.status || error?.response?.status;
        if (status === 404) {
            return {submissions: [], pagination: {page: 1, total: 0}};
        }
        console.error('Failed to fetch solutions:', error);
        return {submissions: [] as SubmissionsListItemModel[], pagination: {page: 1, total: 0}};
    }
}

export async function listUsers(page: number = 1, pageSize: number = 10, search?: string, role?: string) {
    try {
        const response = await Call((client) => client.default.listUsers({page, pageSize, search, role}));
        return response;
    } catch (error) {
        console.error('Failed to fetch users:', error);
        return null;
    }
}

export async function getUser(userId: string) {
    try {
        const response = await Call((client) => client.default.getUser({id: userId}));
        return response;
    } catch (error) {
        console.error('Failed to fetch user:', error);
        return null;
    }
}

export async function getContest(contestId: string) {
    try {
        const response = await Call((client) => client.default.getContest({contestId}));
        return response;
    } catch (error) {
        console.error('Failed to fetch contest:', error);
        return null;
    }
}

export async function getContestProblem(problemId: string, contestId: string) {
    try {
        const response = await Call((client) => client.default.getContestProblem({problemId, contestId}));
        return response;
    } catch (error) {
        console.error('Failed to fetch contest problem:', error);
        return null;
    }
}

export async function getContestMembers(contestId: string, page: number = 1, pageSize: number = 10): Promise<ListContestMembersResponseModel> {
    try {
        const response = await Call((client) => client.default.listContestMembers({contestId, page, pageSize}));
        return response;
    } catch (error) {
        console.error('Failed to fetch participants:', error);
        return {members: [], pagination: {page: 1, total: 0}};
    }
}

export async function getProblem(problemId: string) {
    try {
        const response = await Call((client) => client.default.getProblem({id: problemId}));
        return response;
    } catch (error) {
        console.error('Failed to fetch problem:', error);
        return null;
    }
}

export async function getSubmission(submissionId: string) {
    try {
        const response = await Call((client) => client.default.getSubmission({submissionId}));
        return response;
    } catch (error) {
        console.error('Failed to fetch solution:', error);
        return null;
    }
}

export async function createContest(title: string) {
    try {
        const response = await Call((client) =>
            client.default.createContest({title})
        );
        return response;
    } catch (error) {
        console.error('Failed to create contest:', error);
        throw error;
    }
}

export async function createProblem(title: string) {
    try {
        const response = await Call((client) =>
            client.default.createProblem({title})
        );
        return response;
    } catch (error) {
        console.error('Failed to create problem:', error);
        throw error;
    }
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
    try {
        console.log('📤 Updating problem:', problemId, 'with data:', JSON.stringify(data, null, 2));
        const response = await Call((client) =>
            client.default.updateProblem({id: problemId, requestBody: data})
        );
        return response;
    } catch (error) {
        console.error('Failed to update problem:', error);
        throw error;
    }
}

export async function uploadProblemTests(id: string, file: File) {
    try {
        const response = await Call((client) =>
            client.default.uploadProblemTests({id, formData: {file: file}})
        );
        return response;
    } catch (error) {
        console.error('Failed to upload problem tests:', error);
        throw error;
    }
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
    try {
        console.log('📤 Updating contest:', contestId, 'with data:', JSON.stringify(data, null, 2));
        const response = await Call((client) =>
            client.default.updateContest({contestId, requestBody: data})
        );
        return response;
    } catch (error) {
        console.error('Failed to update contest:', error);
        throw error;
    }
}

export async function addContestProblem(
    contestId: string,
    problemId: string
) {
    try {
        console.log('📤 Adding problem to contest:', {contestId, problemId});
        const response = await Call((client) =>
            client.default.createContestProblem({contestId, problemId})
        );
        return response;
    } catch (error) {
        console.error('Failed to add problem to contest:', error);
        throw error;
    }
}

export async function removeContestProblem(
    contestId: string,
    problemId: string
) {
    try {
        console.log('📤 Removing problem from contest:', {contestId, problemId});
        const response = await Call((client) =>
            client.default.deleteContestProblem({problemId, contestId})
        );
        return response;
    } catch (error) {
        console.error('Failed to remove problem from contest:', error);
        throw error;
    }
}

export async function addContestMember(
    contestId: string,
    userId: string
) {
    try {
        console.log('📤 Adding participant to contest:', {contestId, userId});
        const response = await Call((client) =>
            client.default.createContestMember({contestId, userId})
        );
        return response;
    } catch (error) {
        console.error('Failed to add participant to contest:', error);
        throw error;
    }
}

export async function removeContestMember(
    contestId: string,
    userId: string
) {
    try {
        console.log('📤 Removing participant from contest:', {contestId, userId});
        const response = await Call((client) =>
            client.default.deleteContestMember({userId, contestId})
        );
        return response;
    } catch (error) {
        console.error('Failed to remove participant from contest:', error);
        throw error;
    }
}

export async function searchProblems(title: string, owner?: boolean) {
    try {
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

        const response = await Call((client) => client.default.listProblems(params));
        return response;
    } catch (error) {
        console.error('Failed to search problems:', error);
        return null;
    }
}

// NOTE: duplicate of listUsers
export async function searchUsers(search: string): Promise<ListUsersResponseModel> {
    try {
        const response = await Call((client) =>
            client.default.listUsers({
                page: 1,
                pageSize: 10,
                search: search,
            })
        );
        return response;
    } catch (error) {
        console.error('Failed to search users:', error);
        return {users: [], pagination: {page: 1, total: 0}};
    }
}

export async function createSolution(
    problemId: string,
    contestId: string,
    language: number,
    submission: FormData
) {
    try {
        const solutionData = submission.get("submission");
        let solutionContent: string;
        
        if (solutionData instanceof File) {
            solutionContent = await solutionData.text();
        } else if (typeof solutionData === "string") {
            solutionContent = solutionData;
        } else {
            throw new Error("Invalid solution data type");
        }
        
        console.log("Creating submission:", {problemId, contestId, language});
        const response = await Call((client) =>
            client.default.createSubmission({
                problemId,
                contestId,
                language,
                requestBody: {
                    submission: solutionContent,
                },
            })
        );
        return response;
    } catch (error) {
        console.error('Failed to create solution:', error);
        throw error;
    }
}

// FIXME: Implement actual API endpoint for updating contest member role
export async function updateContestMemberRole(
  contestId: string,
  userId: string,
  newRole: string
): Promise<void> {
  // TODO: Add API call when backend endpoint is ready
  console.log("Update role:", { contestId, userId, newRole });
  console.log("Not implemented!");
}