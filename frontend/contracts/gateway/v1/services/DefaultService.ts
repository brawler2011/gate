/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { CompileResult } from '../models/CompileResult';
import type { CreatedPost } from '../models/CreatedPost';
import type { CreateSubmissionRequestModel } from '../models/CreateSubmissionRequestModel';
import type { CreationResponseModel } from '../models/CreationResponseModel';
import type { GetContestProblemResponseModel } from '../models/GetContestProblemResponseModel';
import type { GetContestResponseModel } from '../models/GetContestResponseModel';
import type { GetHealthResponseModel } from '../models/GetHealthResponseModel';
import type { GetMyContestRoleResponseModel } from '../models/GetMyContestRoleResponseModel';
import type { GetOrganizationResponseModel } from '../models/GetOrganizationResponseModel';
import type { GetProblemResponseModel } from '../models/GetProblemResponseModel';
import type { GetSubmissionResponseModel } from '../models/GetSubmissionResponseModel';
import type { GetTeamResponseModel } from '../models/GetTeamResponseModel';
import type { GetUserResponseModel } from '../models/GetUserResponseModel';
import type { ListContestMembersResponseModel } from '../models/ListContestMembersResponseModel';
import type { ListContestsResponseModel } from '../models/ListContestsResponseModel';
import type { ListOrganizationMembersResponseModel } from '../models/ListOrganizationMembersResponseModel';
import type { ListOrganizationsResponseModel } from '../models/ListOrganizationsResponseModel';
import type { ListPostsResponseModel } from '../models/ListPostsResponseModel';
import type { ListProblemsResponseModel } from '../models/ListProblemsResponseModel';
import type { ListSubmissionsResponseModel } from '../models/ListSubmissionsResponseModel';
import type { ListTeamMembersResponseModel } from '../models/ListTeamMembersResponseModel';
import type { ListTeamsResponseModel } from '../models/ListTeamsResponseModel';
import type { ListUserContestsResponseModel } from '../models/ListUserContestsResponseModel';
import type { ListUsersResponseModel } from '../models/ListUsersResponseModel';
import type { MainComponentSelectionRequest } from '../models/MainComponentSelectionRequest';
import type { MessageResponse } from '../models/MessageResponse';
import type { PostModel } from '../models/PostModel';
import type { ProblemLimits } from '../models/ProblemLimits';
import type { ProblemStatement } from '../models/ProblemStatement';
import type { TestReport } from '../models/TestReport';
import type { UpdateContestRequestModel } from '../models/UpdateContestRequestModel';
import type { UpdateOrganizationRequestModel } from '../models/UpdateOrganizationRequestModel';
import type { UpdateProblemLimitsRequest } from '../models/UpdateProblemLimitsRequest';
import type { UpdateProblemRequestModel } from '../models/UpdateProblemRequestModel';
import type { UpdateProblemStatementRequest } from '../models/UpdateProblemStatementRequest';
import type { UpdateProblemTestsConfigRequest } from '../models/UpdateProblemTestsConfigRequest';
import type { UpdateTeamRequestModel } from '../models/UpdateTeamRequestModel';
import type { ValidationReport } from '../models/ValidationReport';
import type { WorkshopFileListResponse } from '../models/WorkshopFileListResponse';
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
        organizationId,
    }: {
        page: number,
        pageSize: number,
        search?: string,
        descending?: boolean,
        owner?: boolean,
        organizationId?: string,
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
                'organization_id': organizationId,
            },
        });
    }
    /**
     * @returns CreationResponseModel OK
     * @throws ApiError
     */
    public createProblem({
        title,
        organizationId,
    }: {
        title: string,
        organizationId?: string,
    }): CancelablePromise<CreationResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/problems',
            query: {
                'title': title,
                'organization_id': organizationId,
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
        organizationId,
    }: {
        title: string,
        organizationId?: string,
    }): CancelablePromise<CreationResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/contests',
            query: {
                'title': title,
                'organization_id': organizationId,
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
        organizationId,
    }: {
        page: number,
        pageSize: number,
        search?: string,
        sortBy?: 'created_at' | 'updated_at' | 'title',
        sortOrder?: 'asc' | 'desc',
        organizationId?: string,
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
                'organization_id': organizationId,
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
    /**
     * Get user avatar by user ID
     * @returns string User avatar
     * @throws ApiError
     */
    public getUserAvatar({
        id,
        ifNoneMatch,
    }: {
        id: string,
        ifNoneMatch?: string,
    }): CancelablePromise<string> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/users/{id}/avatar',
            path: {
                'id': id,
            },
            headers: {
                'If-None-Match': ifNoneMatch,
            },
            responseHeader: 'ETag',
            errors: {
                304: `Not modified`,
                400: `bad request`,
                404: `not found`,
            },
        });
    }
    /**
     * Upload user avatar
     * @returns any Avatar uploaded successfully
     * @throws ApiError
     */
    public uploadAvatar({
        id,
        formData,
    }: {
        id: string,
        formData: {
            avatar?: Blob;
        },
    }): CancelablePromise<{
        imgId?: string;
    }> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/users/{id}/avatar',
            path: {
                'id': id,
            },
            formData: formData,
            mediaType: 'multipart/form-data',
        });
    }
    /**
     * Delete user avatar
     * @returns any Avatar deleted successfully
     * @throws ApiError
     */
    public deleteAvatar({
        id,
    }: {
        id: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/users/{id}/avatar',
            path: {
                'id': id,
            },
        });
    }
    /**
     * Import package into existing problem
     * @returns any Problem imported successfully
     * @throws ApiError
     */
    public importProblem({
        id,
        formData,
    }: {
        id: string,
        formData: {
            /**
             * Problem package archive (zip)
             */
            package?: Blob;
        },
    }): CancelablePromise<{
        message?: string;
    }> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/problems/{id}/import',
            path: {
                'id': id,
            },
            formData: formData,
            mediaType: 'multipart/form-data',
        });
    }
    /**
     * Publish problem package
     * @returns any Problem published successfully
     * @throws ApiError
     */
    public publishProblem({
        id,
    }: {
        id: string,
    }): CancelablePromise<{
        version?: number;
        message?: string;
    }> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/problems/{id}/publish',
            path: {
                'id': id,
            },
        });
    }
    /**
     * List all packages for a problem
     * @returns any List of problem packages
     * @throws ApiError
     */
    public listProblemPackages({
        id,
    }: {
        id: string,
    }): CancelablePromise<{
        packages?: Array<{
            id?: string;
            version?: number;
            status?: string;
            package_hash?: string;
            created_at?: string;
            compiled_at?: string;
        }>;
    }> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{id}/packages',
            path: {
                'id': id,
            },
        });
    }
    /**
     * Get redirect to published problem package
     * @returns void
     * @throws ApiError
     */
    public getPublishedPackage({
        id,
        version,
    }: {
        id: string,
        version: string,
    }): CancelablePromise<void> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{id}/package/{version}',
            path: {
                'id': id,
                'version': version,
            },
            errors: {
                302: `Redirect to package download URL`,
            },
        });
    }
    /**
     * Initialize problem workspace
     * @returns MessageResponse Workshop initialized successfully
     * @throws ApiError
     */
    public initProblemWorkshop({
        problemId,
    }: {
        problemId: string,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/problems/{problemId}/workshop/init',
            path: {
                'problemId': problemId,
            },
        });
    }
    /**
     * Get problem README
     * @returns binary README content
     * @throws ApiError
     */
    public getProblemReadme({
        problemId,
    }: {
        problemId: string,
    }): CancelablePromise<Blob> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{problemId}/readme',
            path: {
                'problemId': problemId,
            },
        });
    }
    /**
     * Update problem README
     * @returns MessageResponse README updated successfully
     * @throws ApiError
     */
    public updateProblemReadme({
        problemId,
        requestBody,
    }: {
        problemId: string,
        requestBody: Blob,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'PUT',
            url: '/problems/{problemId}/readme',
            path: {
                'problemId': problemId,
            },
            body: requestBody,
            mediaType: 'application/octet-stream',
        });
    }
    /**
     * Get problem limits and type settings
     * @returns ProblemLimits Problem limits
     * @throws ApiError
     */
    public getProblemLimits({
        problemId,
    }: {
        problemId: string,
    }): CancelablePromise<ProblemLimits> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{problemId}/limits',
            path: {
                'problemId': problemId,
            },
        });
    }
    /**
     * Update problem limits and type settings
     * @returns ProblemLimits Limits updated successfully
     * @throws ApiError
     */
    public updateProblemLimits({
        problemId,
        requestBody,
    }: {
        problemId: string,
        requestBody: UpdateProblemLimitsRequest,
    }): CancelablePromise<ProblemLimits> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/problems/{problemId}/limits',
            path: {
                'problemId': problemId,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * Get problem statement
     * @returns ProblemStatement Problem statement
     * @throws ApiError
     */
    public getProblemStatement({
        problemId,
    }: {
        problemId: string,
    }): CancelablePromise<ProblemStatement> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{problemId}/statement',
            path: {
                'problemId': problemId,
            },
        });
    }
    /**
     * Update problem statement
     * @returns ProblemStatement Statement updated successfully
     * @throws ApiError
     */
    public updateProblemStatement({
        problemId,
        requestBody,
    }: {
        problemId: string,
        requestBody: UpdateProblemStatementRequest,
    }): CancelablePromise<ProblemStatement> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/problems/{problemId}/statement',
            path: {
                'problemId': problemId,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * List checker files
     * @returns WorkshopFileListResponse List of checkers
     * @throws ApiError
     */
    public listProblemCheckers({
        problemId,
    }: {
        problemId: string,
    }): CancelablePromise<WorkshopFileListResponse> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{problemId}/checkers',
            path: {
                'problemId': problemId,
            },
        });
    }
    /**
     * Create checker file
     * @returns MessageResponse Checker created successfully
     * @throws ApiError
     */
    public createProblemChecker({
        problemId,
        name,
        requestBody,
    }: {
        problemId: string,
        name: string,
        requestBody: Blob,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/problems/{problemId}/checkers',
            path: {
                'problemId': problemId,
            },
            query: {
                'name': name,
            },
            body: requestBody,
            mediaType: 'application/octet-stream',
        });
    }
    /**
     * Get checker file content
     * @returns binary Checker content
     * @throws ApiError
     */
    public getProblemChecker({
        problemId,
        name,
    }: {
        problemId: string,
        name: string,
    }): CancelablePromise<Blob> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{problemId}/checkers/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
        });
    }
    /**
     * Update checker file
     * @returns MessageResponse Checker updated successfully
     * @throws ApiError
     */
    public updateProblemChecker({
        problemId,
        name,
        requestBody,
    }: {
        problemId: string,
        name: string,
        requestBody: Blob,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'PUT',
            url: '/problems/{problemId}/checkers/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
            body: requestBody,
            mediaType: 'application/octet-stream',
        });
    }
    /**
     * Delete checker file
     * @returns MessageResponse Checker deleted successfully
     * @throws ApiError
     */
    public deleteProblemChecker({
        problemId,
        name,
    }: {
        problemId: string,
        name: string,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/problems/{problemId}/checkers/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
        });
    }
    /**
     * Set main checker file
     * @returns MessageResponse Main checker selected successfully
     * @throws ApiError
     */
    public setProblemCheckerMain({
        problemId,
        requestBody,
    }: {
        problemId: string,
        requestBody: MainComponentSelectionRequest,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/problems/{problemId}/checkers/main',
            path: {
                'problemId': problemId,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * List generator files
     * @returns WorkshopFileListResponse List of generators
     * @throws ApiError
     */
    public listProblemGenerators({
        problemId,
    }: {
        problemId: string,
    }): CancelablePromise<WorkshopFileListResponse> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{problemId}/generators',
            path: {
                'problemId': problemId,
            },
        });
    }
    /**
     * Create generator file
     * @returns MessageResponse Generator created successfully
     * @throws ApiError
     */
    public createProblemGenerator({
        problemId,
        name,
        requestBody,
    }: {
        problemId: string,
        name: string,
        requestBody: Blob,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/problems/{problemId}/generators',
            path: {
                'problemId': problemId,
            },
            query: {
                'name': name,
            },
            body: requestBody,
            mediaType: 'application/octet-stream',
        });
    }
    /**
     * Get generator file content
     * @returns binary Generator content
     * @throws ApiError
     */
    public getProblemGenerator({
        problemId,
        name,
    }: {
        problemId: string,
        name: string,
    }): CancelablePromise<Blob> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{problemId}/generators/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
        });
    }
    /**
     * Update generator file
     * @returns MessageResponse Generator updated successfully
     * @throws ApiError
     */
    public updateProblemGenerator({
        problemId,
        name,
        requestBody,
    }: {
        problemId: string,
        name: string,
        requestBody: Blob,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'PUT',
            url: '/problems/{problemId}/generators/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
            body: requestBody,
            mediaType: 'application/octet-stream',
        });
    }
    /**
     * Delete generator file
     * @returns MessageResponse Generator deleted successfully
     * @throws ApiError
     */
    public deleteProblemGenerator({
        problemId,
        name,
    }: {
        problemId: string,
        name: string,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/problems/{problemId}/generators/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
        });
    }
    /**
     * Set main generator file
     * @returns MessageResponse Main generator selected successfully
     * @throws ApiError
     */
    public setProblemGeneratorMain({
        problemId,
        requestBody,
    }: {
        problemId: string,
        requestBody: MainComponentSelectionRequest,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/problems/{problemId}/generators/main',
            path: {
                'problemId': problemId,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * List interactor files
     * @returns WorkshopFileListResponse List of interactors
     * @throws ApiError
     */
    public listProblemInteractors({
        problemId,
    }: {
        problemId: string,
    }): CancelablePromise<WorkshopFileListResponse> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{problemId}/interactors',
            path: {
                'problemId': problemId,
            },
        });
    }
    /**
     * Create interactor file
     * @returns MessageResponse Interactor created successfully
     * @throws ApiError
     */
    public createProblemInteractor({
        problemId,
        name,
        requestBody,
    }: {
        problemId: string,
        name: string,
        requestBody: Blob,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/problems/{problemId}/interactors',
            path: {
                'problemId': problemId,
            },
            query: {
                'name': name,
            },
            body: requestBody,
            mediaType: 'application/octet-stream',
        });
    }
    /**
     * Get interactor file content
     * @returns binary Interactor content
     * @throws ApiError
     */
    public getProblemInteractor({
        problemId,
        name,
    }: {
        problemId: string,
        name: string,
    }): CancelablePromise<Blob> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{problemId}/interactors/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
        });
    }
    /**
     * Update interactor file
     * @returns MessageResponse Interactor updated successfully
     * @throws ApiError
     */
    public updateProblemInteractor({
        problemId,
        name,
        requestBody,
    }: {
        problemId: string,
        name: string,
        requestBody: Blob,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'PUT',
            url: '/problems/{problemId}/interactors/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
            body: requestBody,
            mediaType: 'application/octet-stream',
        });
    }
    /**
     * Delete interactor file
     * @returns MessageResponse Interactor deleted successfully
     * @throws ApiError
     */
    public deleteProblemInteractor({
        problemId,
        name,
    }: {
        problemId: string,
        name: string,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/problems/{problemId}/interactors/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
        });
    }
    /**
     * Set main interactor file
     * @returns MessageResponse Main interactor selected successfully
     * @throws ApiError
     */
    public setProblemInteractorMain({
        problemId,
        requestBody,
    }: {
        problemId: string,
        requestBody: MainComponentSelectionRequest,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/problems/{problemId}/interactors/main',
            path: {
                'problemId': problemId,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * List media files
     * @returns WorkshopFileListResponse List of media files
     * @throws ApiError
     */
    public listProblemMediaFiles({
        problemId,
    }: {
        problemId: string,
    }): CancelablePromise<WorkshopFileListResponse> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{problemId}/media',
            path: {
                'problemId': problemId,
            },
        });
    }
    /**
     * Create media file
     * @returns MessageResponse Media file created successfully
     * @throws ApiError
     */
    public createProblemMediaFile({
        problemId,
        name,
        requestBody,
    }: {
        problemId: string,
        name: string,
        requestBody: Blob,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/problems/{problemId}/media',
            path: {
                'problemId': problemId,
            },
            query: {
                'name': name,
            },
            body: requestBody,
            mediaType: 'application/octet-stream',
        });
    }
    /**
     * Get media file content
     * @returns binary Media file content
     * @throws ApiError
     */
    public getProblemMediaFile({
        problemId,
        name,
    }: {
        problemId: string,
        name: string,
    }): CancelablePromise<Blob> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{problemId}/media/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
        });
    }
    /**
     * Update media file
     * @returns MessageResponse Media file updated successfully
     * @throws ApiError
     */
    public updateProblemMediaFile({
        problemId,
        name,
        requestBody,
    }: {
        problemId: string,
        name: string,
        requestBody: Blob,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'PUT',
            url: '/problems/{problemId}/media/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
            body: requestBody,
            mediaType: 'application/octet-stream',
        });
    }
    /**
     * Delete media file
     * @returns MessageResponse Media file deleted successfully
     * @throws ApiError
     */
    public deleteProblemMediaFile({
        problemId,
        name,
    }: {
        problemId: string,
        name: string,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/problems/{problemId}/media/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
        });
    }
    /**
     * List author solution files
     * @returns WorkshopFileListResponse List of author solutions
     * @throws ApiError
     */
    public listProblemWorkshopSubmissions({
        problemId,
    }: {
        problemId: string,
    }): CancelablePromise<WorkshopFileListResponse> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{problemId}/submissions',
            path: {
                'problemId': problemId,
            },
        });
    }
    /**
     * Create author solution file
     * @returns MessageResponse Author solution file created successfully
     * @throws ApiError
     */
    public createProblemWorkshopSubmission({
        problemId,
        name,
        requestBody,
    }: {
        problemId: string,
        name: string,
        requestBody: Blob,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/problems/{problemId}/submissions',
            path: {
                'problemId': problemId,
            },
            query: {
                'name': name,
            },
            body: requestBody,
            mediaType: 'application/octet-stream',
        });
    }
    /**
     * Get author solution file content
     * @returns binary Author solution file content
     * @throws ApiError
     */
    public getProblemWorkshopSubmission({
        problemId,
        name,
    }: {
        problemId: string,
        name: string,
    }): CancelablePromise<Blob> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{problemId}/submissions/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
        });
    }
    /**
     * Update author solution file
     * @returns MessageResponse Author solution file updated successfully
     * @throws ApiError
     */
    public updateProblemWorkshopSubmission({
        problemId,
        name,
        requestBody,
    }: {
        problemId: string,
        name: string,
        requestBody: Blob,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'PUT',
            url: '/problems/{problemId}/submissions/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
            body: requestBody,
            mediaType: 'application/octet-stream',
        });
    }
    /**
     * Delete author solution file
     * @returns MessageResponse Author solution file deleted successfully
     * @throws ApiError
     */
    public deleteProblemWorkshopSubmission({
        problemId,
        name,
    }: {
        problemId: string,
        name: string,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/problems/{problemId}/submissions/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
        });
    }
    /**
     * List test files
     * @returns WorkshopFileListResponse List of tests
     * @throws ApiError
     */
    public listProblemTests({
        problemId,
    }: {
        problemId: string,
    }): CancelablePromise<WorkshopFileListResponse> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{problemId}/tests',
            path: {
                'problemId': problemId,
            },
        });
    }
    /**
     * Create test file
     * @returns MessageResponse Test file created successfully
     * @throws ApiError
     */
    public createProblemTestFile({
        problemId,
        name,
        requestBody,
    }: {
        problemId: string,
        name: string,
        requestBody: Blob,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/problems/{problemId}/tests',
            path: {
                'problemId': problemId,
            },
            query: {
                'name': name,
            },
            body: requestBody,
            mediaType: 'application/octet-stream',
        });
    }
    /**
     * Update tests.json configuration
     * @returns MessageResponse Tests config updated successfully
     * @throws ApiError
     */
    public updateProblemTestsConfig({
        problemId,
        requestBody,
    }: {
        problemId: string,
        requestBody: UpdateProblemTestsConfigRequest,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/problems/{problemId}/tests/config',
            path: {
                'problemId': problemId,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * Get test file content
     * @returns binary Test file content
     * @throws ApiError
     */
    public getProblemTestFile({
        problemId,
        name,
    }: {
        problemId: string,
        name: string,
    }): CancelablePromise<Blob> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{problemId}/tests/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
        });
    }
    /**
     * Update test file
     * @returns MessageResponse Test file updated successfully
     * @throws ApiError
     */
    public updateProblemTestFile({
        problemId,
        name,
        requestBody,
    }: {
        problemId: string,
        name: string,
        requestBody: Blob,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'PUT',
            url: '/problems/{problemId}/tests/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
            body: requestBody,
            mediaType: 'application/octet-stream',
        });
    }
    /**
     * Delete test file
     * @returns MessageResponse Test file deleted successfully
     * @throws ApiError
     */
    public deleteProblemTestFile({
        problemId,
        name,
    }: {
        problemId: string,
        name: string,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/problems/{problemId}/tests/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
        });
    }
    /**
     * List validator files
     * @returns WorkshopFileListResponse List of validators
     * @throws ApiError
     */
    public listProblemValidators({
        problemId,
    }: {
        problemId: string,
    }): CancelablePromise<WorkshopFileListResponse> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{problemId}/validators',
            path: {
                'problemId': problemId,
            },
        });
    }
    /**
     * Create validator file
     * @returns MessageResponse Validator created successfully
     * @throws ApiError
     */
    public createProblemValidator({
        problemId,
        name,
        requestBody,
    }: {
        problemId: string,
        name: string,
        requestBody: Blob,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/problems/{problemId}/validators',
            path: {
                'problemId': problemId,
            },
            query: {
                'name': name,
            },
            body: requestBody,
            mediaType: 'application/octet-stream',
        });
    }
    /**
     * Get validator file content
     * @returns binary Validator content
     * @throws ApiError
     */
    public getProblemValidator({
        problemId,
        name,
    }: {
        problemId: string,
        name: string,
    }): CancelablePromise<Blob> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{problemId}/validators/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
        });
    }
    /**
     * Update validator file
     * @returns MessageResponse Validator updated successfully
     * @throws ApiError
     */
    public updateProblemValidator({
        problemId,
        name,
        requestBody,
    }: {
        problemId: string,
        name: string,
        requestBody: Blob,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'PUT',
            url: '/problems/{problemId}/validators/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
            body: requestBody,
            mediaType: 'application/octet-stream',
        });
    }
    /**
     * Delete validator file
     * @returns MessageResponse Validator deleted successfully
     * @throws ApiError
     */
    public deleteProblemValidator({
        problemId,
        name,
    }: {
        problemId: string,
        name: string,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/problems/{problemId}/validators/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
        });
    }
    /**
     * Set main validator file
     * @returns MessageResponse Main validator selected successfully
     * @throws ApiError
     */
    public setProblemValidatorMain({
        problemId,
        requestBody,
    }: {
        problemId: string,
        requestBody: MainComponentSelectionRequest,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/problems/{problemId}/validators/main',
            path: {
                'problemId': problemId,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * Compile checker/validator/generator/interactor
     * @returns CompileResult Compilation result
     * @throws ApiError
     */
    public compileProblemComponent({
        problemId,
        componentType,
    }: {
        problemId: string,
        componentType: 'checker' | 'validator' | 'generator' | 'interactor',
    }): CancelablePromise<CompileResult> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/problems/{problemId}/workshop/components/{componentType}/compile',
            path: {
                'problemId': problemId,
                'componentType': componentType,
            },
        });
    }
    /**
     * Generate tests using generator
     * @returns any Tests generated successfully
     * @throws ApiError
     */
    public generateTests({
        problemId,
        requestBody,
    }: {
        problemId: string,
        requestBody: {
            generator_name: string;
            test_numbers: Array<number>;
            arguments?: Array<Array<string>>;
        },
    }): CancelablePromise<{
        message?: string;
    }> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/problems/{problemId}/workshop/tests/generate',
            path: {
                'problemId': problemId,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * Validate all test inputs
     * @returns ValidationReport Validation report
     * @throws ApiError
     */
    public validateAllTests({
        problemId,
    }: {
        problemId: string,
    }): CancelablePromise<ValidationReport> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/problems/{problemId}/workshop/tests/validate',
            path: {
                'problemId': problemId,
            },
        });
    }
    /**
     * Test solution against tests
     * @returns TestReport Test report
     * @throws ApiError
     */
    public testSolution({
        problemId,
        requestBody,
    }: {
        problemId: string,
        requestBody: {
            /**
             * Path to solution file in repository
             */
            solution_path: string;
            /**
             * Specific test numbers to run (empty = all tests)
             */
            test_numbers?: Array<number>;
        },
    }): CancelablePromise<TestReport> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/problems/{problemId}/workshop/solutions/test',
            path: {
                'problemId': problemId,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * List organizations
     * @returns ListOrganizationsResponseModel List of organizations
     * @throws ApiError
     */
    public listOrganizations({
        page,
        pageSize,
        search,
    }: {
        page: number,
        pageSize: number,
        search?: string,
    }): CancelablePromise<ListOrganizationsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/organizations',
            query: {
                'page': page,
                'pageSize': pageSize,
                'search': search,
            },
        });
    }
    /**
     * Create a new organization
     * @returns CreationResponseModel Organization created successfully
     * @throws ApiError
     */
    public createOrganization({
        name,
    }: {
        name: string,
    }): CancelablePromise<CreationResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations',
            query: {
                'name': name,
            },
        });
    }
    /**
     * Get organization by ID
     * @returns GetOrganizationResponseModel Organization details
     * @throws ApiError
     */
    public getOrganization({
        id,
    }: {
        id: string,
    }): CancelablePromise<GetOrganizationResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/organizations/{id}',
            path: {
                'id': id,
            },
        });
    }
    /**
     * Update organization
     * @returns any Organization updated successfully
     * @throws ApiError
     */
    public updateOrganization({
        id,
        requestBody,
    }: {
        id: string,
        requestBody: UpdateOrganizationRequestModel,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/organizations/{id}',
            path: {
                'id': id,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * Delete organization
     * @returns any Organization deleted successfully
     * @throws ApiError
     */
    public deleteOrganization({
        id,
    }: {
        id: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/organizations/{id}',
            path: {
                'id': id,
            },
        });
    }
    /**
     * List organization members
     * @returns ListOrganizationMembersResponseModel List of organization members
     * @throws ApiError
     */
    public listOrganizationMembers({
        id,
        page,
        pageSize,
    }: {
        id: string,
        page: number,
        pageSize: number,
    }): CancelablePromise<ListOrganizationMembersResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/organizations/{id}/members',
            path: {
                'id': id,
            },
            query: {
                'page': page,
                'pageSize': pageSize,
            },
        });
    }
    /**
     * Add member to organization
     * @returns any Member added successfully
     * @throws ApiError
     */
    public addOrganizationMember({
        id,
        userId,
        role,
    }: {
        id: string,
        userId: string,
        role: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations/{id}/members',
            path: {
                'id': id,
            },
            query: {
                'user_id': userId,
                'role': role,
            },
        });
    }
    /**
     * Remove member from organization
     * @returns any Member removed successfully
     * @throws ApiError
     */
    public removeOrganizationMember({
        id,
        userId,
    }: {
        id: string,
        userId: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/organizations/{id}/members',
            path: {
                'id': id,
            },
            query: {
                'user_id': userId,
            },
        });
    }
    /**
     * List teams
     * @returns ListTeamsResponseModel List of teams
     * @throws ApiError
     */
    public listTeams({
        page,
        pageSize,
        search,
        organizationId,
    }: {
        page: number,
        pageSize: number,
        search?: string,
        organizationId?: string,
    }): CancelablePromise<ListTeamsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/teams',
            query: {
                'page': page,
                'pageSize': pageSize,
                'search': search,
                'organization_id': organizationId,
            },
        });
    }
    /**
     * Create a new team
     * @returns CreationResponseModel Team created successfully
     * @throws ApiError
     */
    public createTeam({
        requestBody,
    }: {
        requestBody: {
            name: string;
            organization_id: string;
        },
    }): CancelablePromise<CreationResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/teams',
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * Get team by ID
     * @returns GetTeamResponseModel Team details
     * @throws ApiError
     */
    public getTeam({
        id,
    }: {
        id: string,
    }): CancelablePromise<GetTeamResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/teams/{id}',
            path: {
                'id': id,
            },
        });
    }
    /**
     * Update team
     * @returns any Team updated successfully
     * @throws ApiError
     */
    public updateTeam({
        id,
        requestBody,
    }: {
        id: string,
        requestBody: UpdateTeamRequestModel,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/teams/{id}',
            path: {
                'id': id,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * Delete team
     * @returns any Team deleted successfully
     * @throws ApiError
     */
    public deleteTeam({
        id,
    }: {
        id: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/teams/{id}',
            path: {
                'id': id,
            },
        });
    }
    /**
     * List team members
     * @returns ListTeamMembersResponseModel List of team members
     * @throws ApiError
     */
    public listTeamMembers({
        id,
        page,
        pageSize,
    }: {
        id: string,
        page: number,
        pageSize: number,
    }): CancelablePromise<ListTeamMembersResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/teams/{id}/members',
            path: {
                'id': id,
            },
            query: {
                'page': page,
                'pageSize': pageSize,
            },
        });
    }
    /**
     * Add member to team
     * @returns any Member added successfully
     * @throws ApiError
     */
    public addTeamMember({
        id,
        userId,
    }: {
        id: string,
        userId: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/teams/{id}/members',
            path: {
                'id': id,
            },
            query: {
                'user_id': userId,
            },
        });
    }
    /**
     * Remove member from team
     * @returns any Member removed successfully
     * @throws ApiError
     */
    public removeTeamMember({
        id,
        userId,
    }: {
        id: string,
        userId: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/teams/{id}/members',
            path: {
                'id': id,
            },
            query: {
                'user_id': userId,
            },
        });
    }
    /**
     * Get a list of posts
     * @returns ListPostsResponseModel A list of posts
     * @throws ApiError
     */
    public listPosts({
        page = 1,
        pageSize = 10,
        sortOrder = 'desc',
    }: {
        page?: number,
        pageSize?: number,
        sortOrder?: 'asc' | 'desc',
    }): CancelablePromise<ListPostsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/posts',
            query: {
                'page': page,
                'page_size': pageSize,
                'sort_order': sortOrder,
            },
        });
    }
    /**
     * Create a new post
     * @returns CreatedPost Post created successfully
     * @throws ApiError
     */
    public createPost({
        formData,
    }: {
        formData: {
            title?: string;
            description?: string;
            text?: string;
            preview_image?: Blob;
        },
    }): CancelablePromise<CreatedPost> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/posts',
            formData: formData,
            mediaType: 'multipart/form-data',
            errors: {
                400: `bad request`,
                401: `unauthorized`,
                403: `forbidden`,
            },
        });
    }
    /**
     * Get a single post by ID
     * @returns PostModel A single post
     * @throws ApiError
     */
    public getPostById({
        id,
    }: {
        id: string,
    }): CancelablePromise<PostModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/posts/{id}',
            path: {
                'id': id,
            },
            errors: {
                400: `bad request`,
                404: `not found`,
            },
        });
    }
    /**
     * Partially update a post by ID
     * @returns any Post partially updated successfully
     * @throws ApiError
     */
    public patchPostById({
        id,
        formData,
    }: {
        id: string,
        formData?: {
            title?: string;
            description?: string;
            text?: string;
            preview_image?: Blob;
        },
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/posts/{id}',
            path: {
                'id': id,
            },
            formData: formData,
            mediaType: 'multipart/form-data',
            errors: {
                400: `bad request`,
                401: `unauthorized`,
                403: `forbidden`,
                404: `not found`,
            },
        });
    }
    /**
     * Delete a post by ID
     * @returns any Post deleted successfully
     * @throws ApiError
     */
    public deletePostById({
        id,
    }: {
        id: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/posts/{id}',
            path: {
                'id': id,
            },
            errors: {
                400: `bad request`,
                401: `unauthorized`,
                403: `forbidden`,
                404: `not found`,
            },
        });
    }
    /**
     * Get image of the post by ID
     * @returns any Post image
     * @throws ApiError
     */
    public getPostImage({
        id,
    }: {
        id: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/posts/{id}/image',
            path: {
                'id': id,
            },
            errors: {
                400: `bad request`,
                404: `not found`,
            },
        });
    }
}
