/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { CreateSubmissionRequestModel } from '../models/CreateSubmissionRequestModel';
import type { CreationResponseModel } from '../models/CreationResponseModel';
import type { GetContestProblemResponseModel } from '../models/GetContestProblemResponseModel';
import type { GetContestResponseModel } from '../models/GetContestResponseModel';
import type { GetHealthResponseModel } from '../models/GetHealthResponseModel';
import type { GetMyContestRoleResponseModel } from '../models/GetMyContestRoleResponseModel';
import type { GetProblemResponseModel } from '../models/GetProblemResponseModel';
import type { GetSubmissionResponseModel } from '../models/GetSubmissionResponseModel';
import type { GetUserResponseModel } from '../models/GetUserResponseModel';
import type { ListContestMembersResponseModel } from '../models/ListContestMembersResponseModel';
import type { ListContestsResponseModel } from '../models/ListContestsResponseModel';
import type { ListProblemsResponseModel } from '../models/ListProblemsResponseModel';
import type { ListSubmissionsResponseModel } from '../models/ListSubmissionsResponseModel';
import type { ListUserContestsResponseModel } from '../models/ListUserContestsResponseModel';
import type { ListUsersResponseModel } from '../models/ListUsersResponseModel';
import type { UpdateContestRequestModel } from '../models/UpdateContestRequestModel';
import type { UpdateProblemRequestModel } from '../models/UpdateProblemRequestModel';
import type { CancelablePromise } from '../core/CancelablePromise';
import type { BaseHttpRequest } from '../core/BaseHttpRequest';
export class DefaultService {
    constructor(public readonly httpRequest: BaseHttpRequest) {}
    /**
     * @returns ListProblemsResponseModel OK
     * @throws ApiError
     */
    public listProblems({
        page,
        pageSize,
        search,
        descending,
        owner,
    }: {
        page: number,
        pageSize: number,
        search?: string,
        descending?: boolean,
        owner?: boolean,
    }): CancelablePromise<ListProblemsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems',
            query: {
                'page': page,
                'pageSize': pageSize,
                'search': search,
                'descending': descending,
                'owner': owner,
            },
        });
    }
    /**
     * @returns CreationResponseModel OK
     * @throws ApiError
     */
    public createProblem({
        title,
    }: {
        title: string,
    }): CancelablePromise<CreationResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/problems',
            query: {
                'title': title,
            },
        });
    }
    /**
     * @returns GetProblemResponseModel OK
     * @throws ApiError
     */
    public getProblem({
        id,
    }: {
        id: string,
    }): CancelablePromise<GetProblemResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{id}',
            path: {
                'id': id,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public deleteProblem({
        id,
    }: {
        id: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/problems/{id}',
            path: {
                'id': id,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public updateProblem({
        id,
        requestBody,
    }: {
        id: string,
        requestBody: UpdateProblemRequestModel,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/problems/{id}',
            path: {
                'id': id,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * @returns CreationResponseModel OK
     * @throws ApiError
     */
    public createContest({
        title,
    }: {
        title: string,
    }): CancelablePromise<CreationResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/contests',
            query: {
                'title': title,
            },
        });
    }
    /**
     * @returns ListContestsResponseModel OK
     * @throws ApiError
     */
    public listAdminContests({
        page,
        pageSize,
        search,
        visibility,
        sortBy,
        sortOrder,
    }: {
        page: number,
        pageSize: number,
        search?: string,
        visibility?: 'public' | 'private',
        sortBy?: 'created_at' | 'updated_at' | 'title',
        sortOrder?: 'asc' | 'desc',
    }): CancelablePromise<ListContestsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/admin/contests',
            query: {
                'page': page,
                'pageSize': pageSize,
                'search': search,
                'visibility': visibility,
                'sortBy': sortBy,
                'sortOrder': sortOrder,
            },
        });
    }
    /**
     * @returns ListUserContestsResponseModel OK
     * @throws ApiError
     */
    public listUserContests({
        id,
        page,
        pageSize,
        search,
        sortBy,
        sortOrder,
    }: {
        id: string,
        page: number,
        pageSize: number,
        search?: string,
        sortBy?: 'last_submission_time' | 'created_at' | 'updated_at' | 'title',
        sortOrder?: 'asc' | 'desc',
    }): CancelablePromise<ListUserContestsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/user/{id}/contests',
            path: {
                'id': id,
            },
            query: {
                'page': page,
                'pageSize': pageSize,
                'search': search,
                'sortBy': sortBy,
                'sortOrder': sortOrder,
            },
        });
    }
    /**
     * @returns ListContestsResponseModel OK
     * @throws ApiError
     */
    public listWorkshopContests({
        page,
        pageSize,
        search,
        sortBy,
        sortOrder,
    }: {
        page: number,
        pageSize: number,
        search?: string,
        sortBy?: 'created_at' | 'updated_at' | 'title',
        sortOrder?: 'asc' | 'desc',
    }): CancelablePromise<ListContestsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/workshop/contests',
            query: {
                'page': page,
                'pageSize': pageSize,
                'search': search,
                'sortBy': sortBy,
                'sortOrder': sortOrder,
            },
        });
    }
    /**
     * @returns ListContestsResponseModel OK
     * @throws ApiError
     */
    public listPublicContests({
        page,
        pageSize,
        search,
        sortBy,
        sortOrder,
    }: {
        page: number,
        pageSize: number,
        search?: string,
        sortBy?: 'created_at' | 'updated_at' | 'title',
        sortOrder?: 'asc' | 'desc',
    }): CancelablePromise<ListContestsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/public/contests',
            query: {
                'page': page,
                'pageSize': pageSize,
                'search': search,
                'sortBy': sortBy,
                'sortOrder': sortOrder,
            },
        });
    }
    /**
     * @returns GetContestResponseModel OK
     * @throws ApiError
     */
    public getContest({
        contestId,
    }: {
        contestId: string,
    }): CancelablePromise<GetContestResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/contests/{contest_id}',
            path: {
                'contest_id': contestId,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public deleteContest({
        contestId,
    }: {
        contestId: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/contests/{contest_id}',
            path: {
                'contest_id': contestId,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public updateContest({
        contestId,
        requestBody,
    }: {
        contestId: string,
        requestBody: UpdateContestRequestModel,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/contests/{contest_id}',
            path: {
                'contest_id': contestId,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * @returns CreationResponseModel OK
     * @throws ApiError
     */
    public createContestProblem({
        contestId,
        problemId,
    }: {
        contestId: string,
        problemId: string,
    }): CancelablePromise<CreationResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/contests/{contest_id}/problems',
            path: {
                'contest_id': contestId,
            },
            query: {
                'problem_id': problemId,
            },
        });
    }
    /**
     * @returns GetContestProblemResponseModel OK
     * @throws ApiError
     */
    public getContestProblem({
        problemId,
        contestId,
    }: {
        problemId: string,
        contestId: string,
    }): CancelablePromise<GetContestProblemResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/contests/{contest_id}/problems/{problem_id}',
            path: {
                'problem_id': problemId,
                'contest_id': contestId,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public deleteContestProblem({
        problemId,
        contestId,
    }: {
        problemId: string,
        contestId: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/contests/{contest_id}/problems/{problem_id}',
            path: {
                'problem_id': problemId,
                'contest_id': contestId,
            },
        });
    }
    /**
     * @returns ListContestMembersResponseModel OK
     * @throws ApiError
     */
    public listContestMembers({
        contestId,
        page,
        pageSize,
    }: {
        contestId: string,
        page: number,
        pageSize: number,
    }): CancelablePromise<ListContestMembersResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/contests/{contest_id}/members',
            path: {
                'contest_id': contestId,
            },
            query: {
                'page': page,
                'pageSize': pageSize,
            },
        });
    }
    /**
     * @returns CreationResponseModel OK
     * @throws ApiError
     */
    public createContestMember({
        contestId,
        userId,
    }: {
        contestId: string,
        userId: string,
    }): CancelablePromise<CreationResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/contests/{contest_id}/members',
            path: {
                'contest_id': contestId,
            },
            query: {
                'user_id': userId,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public deleteContestMember({
        userId,
        contestId,
    }: {
        userId: string,
        contestId: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/contests/{contest_id}/members',
            path: {
                'contest_id': contestId,
            },
            query: {
                'user_id': userId,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public updateContestMember({
        contestId,
        userId,
        role,
    }: {
        contestId: string,
        userId: string,
        role: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/contests/{contest_id}/members',
            path: {
                'contest_id': contestId,
            },
            query: {
                'user_id': userId,
                'role': role,
            },
        });
    }
    /**
     * @returns GetMyContestRoleResponseModel OK
     * @throws ApiError
     */
    public getMyContestRole({
        contestId,
    }: {
        contestId: string,
    }): CancelablePromise<GetMyContestRoleResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/contests/{contest_id}/my-role',
            path: {
                'contest_id': contestId,
            },
        });
    }
    /**
     * @returns ListSubmissionsResponseModel OK
     * @throws ApiError
     */
    public listContestSubmissions({
        contestId,
        page,
        pageSize,
        userId,
        problemId,
        state,
        sortOrder,
        language,
    }: {
        contestId: string,
        page: number,
        pageSize: number,
        userId?: string,
        problemId?: string,
        state?: number,
        sortOrder?: 'asc' | 'desc',
        language?: number,
    }): CancelablePromise<ListSubmissionsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/contests/{contest_id}/submissions',
            path: {
                'contest_id': contestId,
            },
            query: {
                'page': page,
                'pageSize': pageSize,
                'userId': userId,
                'problemId': problemId,
                'state': state,
                'sortOrder': sortOrder,
                'language': language,
            },
        });
    }
    /**
     * @returns CreationResponseModel OK
     * @throws ApiError
     */
    public createSubmission({
        problemId,
        contestId,
        language,
        requestBody,
    }: {
        problemId: string,
        contestId: string,
        language: number,
        requestBody: CreateSubmissionRequestModel,
    }): CancelablePromise<CreationResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/submissions',
            query: {
                'problem_id': problemId,
                'contest_id': contestId,
                'language': language,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * @returns ListSubmissionsResponseModel OK
     * @throws ApiError
     */
    public listSubmissions({
        page,
        pageSize,
        contestId,
        userId,
        problemId,
        state,
        sortOrder,
        language,
    }: {
        page: number,
        pageSize: number,
        contestId?: string,
        userId?: string,
        problemId?: string,
        state?: number,
        sortOrder?: 'asc' | 'desc',
        language?: number,
    }): CancelablePromise<ListSubmissionsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/submissions',
            query: {
                'page': page,
                'pageSize': pageSize,
                'contestId': contestId,
                'userId': userId,
                'problemId': problemId,
                'state': state,
                'sortOrder': sortOrder,
                'language': language,
            },
        });
    }
    /**
     * @returns GetSubmissionResponseModel OK
     * @throws ApiError
     */
    public getSubmission({
        submissionId,
    }: {
        submissionId: string,
    }): CancelablePromise<GetSubmissionResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/submissions/{submission_id}',
            path: {
                'submission_id': submissionId,
            },
        });
    }
    /**
     * @returns GetHealthResponseModel OK
     * @throws ApiError
     */
    public getHealth(): CancelablePromise<GetHealthResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/health',
        });
    }
    /**
     * @returns ListUsersResponseModel OK
     * @throws ApiError
     */
    public listUsers({
        page,
        pageSize,
        search,
        role,
    }: {
        page: number,
        pageSize: number,
        search?: string,
        role?: string,
    }): CancelablePromise<ListUsersResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/users',
            query: {
                'page': page,
                'pageSize': pageSize,
                'search': search,
                'role': role,
            },
        });
    }
    /**
     * @returns GetUserResponseModel OK
     * @throws ApiError
     */
    public getUser({
        id,
    }: {
        id: string,
    }): CancelablePromise<GetUserResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/users/{id}',
            path: {
                'id': id,
            },
        });
    }
    /**
     * @returns ListSubmissionsResponseModel OK
     * @throws ApiError
     */
    public listUserSubmissions({
        userId,
        page,
        pageSize,
        contestId,
        problemId,
        state,
        sortOrder,
    }: {
        userId: string,
        page: number,
        pageSize: number,
        contestId?: string,
        problemId?: string,
        state?: number,
        sortOrder?: 'asc' | 'desc',
    }): CancelablePromise<ListSubmissionsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/users/{user_id}/submissions',
            path: {
                'user_id': userId,
            },
            query: {
                'page': page,
                'pageSize': pageSize,
                'contestId': contestId,
                'problemId': problemId,
                'state': state,
                'sortOrder': sortOrder,
            },
        });
    }
    /**
     * @returns GetUserResponseModel OK
     * @throws ApiError
     */
    public getMe(): CancelablePromise<GetUserResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/users/me',
        });
    }
}
