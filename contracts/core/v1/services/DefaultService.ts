/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { Commit } from '../models/Commit';
import type { CompileResult } from '../models/CompileResult';
import type { CreatedPost } from '../models/CreatedPost';
import type { CreateSubmissionRequestModel } from '../models/CreateSubmissionRequestModel';
import type { CreationResponseModel } from '../models/CreationResponseModel';
import type { FileEntry } from '../models/FileEntry';
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
import type { PostModel } from '../models/PostModel';
import type { TestReport } from '../models/TestReport';
import type { UpdateContestRequestModel } from '../models/UpdateContestRequestModel';
import type { UpdateOrganizationRequestModel } from '../models/UpdateOrganizationRequestModel';
import type { UpdateProblemRequestModel } from '../models/UpdateProblemRequestModel';
import type { UpdateTeamRequestModel } from '../models/UpdateTeamRequestModel';
import type { ValidationReport } from '../models/ValidationReport';
import type { WorkshopStatus } from '../models/WorkshopStatus';
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
     * Import problem from package
     * @returns CreationResponseModel Problem imported successfully
     * @throws ApiError
     */
    public importProblem({
        formData,
    }: {
        formData: {
            /**
             * Problem package archive (zip)
             */
            package?: Blob;
        },
    }): CancelablePromise<CreationResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/problems/import',
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
     * Initialize problem workshop with Git repository
     * @returns any Workshop initialized successfully
     * @throws ApiError
     */
    public initProblemWorkshop({
        problemId,
    }: {
        problemId: string,
    }): CancelablePromise<{
        message?: string;
        commit_sha?: string;
    }> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/problems/{problemId}/workshop/init',
            path: {
                'problemId': problemId,
            },
        });
    }
    /**
     * List files in problem repository
     * @returns any List of files
     * @throws ApiError
     */
    public listWorkshopFiles({
        problemId,
        path = '',
    }: {
        problemId: string,
        /**
         * Directory path to list (empty for root)
         */
        path?: string,
    }): CancelablePromise<{
        files?: Array<FileEntry>;
    }> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{problemId}/workshop/files',
            path: {
                'problemId': problemId,
            },
            query: {
                'path': path,
            },
        });
    }
    /**
     * Read file content from repository
     * @returns binary File content
     * @throws ApiError
     */
    public getWorkshopFile({
        problemId,
        path,
    }: {
        problemId: string,
        path: string,
    }): CancelablePromise<Blob> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{problemId}/workshop/files/{path}',
            path: {
                'problemId': problemId,
                'path': path,
            },
        });
    }
    /**
     * Update file content in repository
     * @returns any File updated successfully
     * @throws ApiError
     */
    public updateWorkshopFile({
        problemId,
        path,
        requestBody,
    }: {
        problemId: string,
        path: string,
        requestBody: Blob,
    }): CancelablePromise<{
        message?: string;
    }> {
        return this.httpRequest.request({
            method: 'PUT',
            url: '/problems/{problemId}/workshop/files/{path}',
            path: {
                'problemId': problemId,
                'path': path,
            },
            body: requestBody,
            mediaType: 'application/octet-stream',
        });
    }
    /**
     * Delete file from repository
     * @returns any File deleted successfully
     * @throws ApiError
     */
    public deleteWorkshopFile({
        problemId,
        path,
    }: {
        problemId: string,
        path: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/problems/{problemId}/workshop/files/{path}',
            path: {
                'problemId': problemId,
                'path': path,
            },
        });
    }
    /**
     * Commit changes to repository
     * @returns any Changes committed successfully
     * @throws ApiError
     */
    public commitWorkshopChanges({
        problemId,
        requestBody,
    }: {
        problemId: string,
        requestBody: {
            /**
             * Commit message
             */
            message: string;
            /**
             * Author name
             */
            author_name?: string;
            /**
             * Author email
             */
            author_email?: string;
        },
    }): CancelablePromise<{
        commit_sha?: string;
        message?: string;
    }> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/problems/{problemId}/workshop/commit',
            path: {
                'problemId': problemId,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * Get current workshop status
     * @returns WorkshopStatus Workshop status
     * @throws ApiError
     */
    public getWorkshopStatus({
        problemId,
    }: {
        problemId: string,
    }): CancelablePromise<WorkshopStatus> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{problemId}/workshop/status',
            path: {
                'problemId': problemId,
            },
        });
    }
    /**
     * Get commit history
     * @returns any Commit history
     * @throws ApiError
     */
    public getWorkshopHistory({
        problemId,
        limit = 20,
    }: {
        problemId: string,
        limit?: number,
    }): CancelablePromise<{
        commits?: Array<Commit>;
    }> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{problemId}/workshop/history',
            path: {
                'problemId': problemId,
            },
            query: {
                'limit': limit,
            },
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
     * @returns string Post image
     * @throws ApiError
     */
    public getPostImage({
        id,
        ifNoneMatch,
    }: {
        id: string,
        ifNoneMatch?: string,
    }): CancelablePromise<string> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/posts/{id}/image',
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
}
