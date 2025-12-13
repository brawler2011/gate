/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { CreatedPost } from '../models/CreatedPost';
import type { CreateSampleRequestModel } from '../models/CreateSampleRequestModel';
import type { CreateSubmissionRequestModel } from '../models/CreateSubmissionRequestModel';
import type { CreationResponseModel } from '../models/CreationResponseModel';
import type { GetContestProblemResponseModel } from '../models/GetContestProblemResponseModel';
import type { GetContestResponseModel } from '../models/GetContestResponseModel';
import type { GetHealthResponseModel } from '../models/GetHealthResponseModel';
import type { GetMonitorResponseModel } from '../models/GetMonitorResponseModel';
import type { GetMyContestRoleResponseModel } from '../models/GetMyContestRoleResponseModel';
import type { GetProblemResponseModel } from '../models/GetProblemResponseModel';
import type { GetSamplesResponseModel } from '../models/GetSamplesResponseModel';
import type { GetSubmissionResponseModel } from '../models/GetSubmissionResponseModel';
import type { GetTestGroupsResponseModel } from '../models/GetTestGroupsResponseModel';
import type { GetUserResponseModel } from '../models/GetUserResponseModel';
import type { ListAccessRequestsResponseModel } from '../models/ListAccessRequestsResponseModel';
import type { ListContestMembersResponseModel } from '../models/ListContestMembersResponseModel';
import type { ListContestsResponseModel } from '../models/ListContestsResponseModel';
import type { ListInvitationsResponseModel } from '../models/ListInvitationsResponseModel';
import type { ListPostsResponseModel } from '../models/ListPostsResponseModel';
import type { ListProblemsResponseModel } from '../models/ListProblemsResponseModel';
import type { ListSubmissionsResponseModel } from '../models/ListSubmissionsResponseModel';
import type { ListUserContestsResponseModel } from '../models/ListUserContestsResponseModel';
import type { ListUsersResponseModel } from '../models/ListUsersResponseModel';
import type { PostModel } from '../models/PostModel';
import type { UpdateContestRequestModel } from '../models/UpdateContestRequestModel';
import type { UpdateProblemRequestModel } from '../models/UpdateProblemRequestModel';
import type { UpdateTestGroupRequestModel } from '../models/UpdateTestGroupRequestModel';
import type { CancelablePromise } from '../core/CancelablePromise';
import type { BaseHttpRequest } from '../core/BaseHttpRequest';
export class DefaultService {
    constructor(public readonly httpRequest: BaseHttpRequest) {}
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
            url: '/blogs/posts/{id}/image',
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
            url: '/blogs/posts',
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
            url: '/blogs/posts',
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
            url: '/blogs/posts/{id}',
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
            url: '/blogs/posts/{id}',
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
            url: '/blogs/posts/{id}',
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
            url: '/tester/problems',
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
            url: '/tester/problems',
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
            url: '/tester/problems/{id}',
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
            url: '/tester/problems/{id}',
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
            url: '/tester/problems/{id}',
            path: {
                'id': id,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public uploadProblemTests({
        id,
        formData,
    }: {
        id: string,
        formData: {
            file: Blob;
        },
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/tester/problems/{id}/tests',
            path: {
                'id': id,
            },
            formData: formData,
            mediaType: 'multipart/form-data',
        });
    }
    /**
     * @returns CreationResponseModel OK
     * @throws ApiError
     */
    public createTestGroup({
        id,
        ordinal,
        name,
        points,
        isSample,
    }: {
        id: string,
        ordinal: number,
        name: string,
        points: number,
        isSample?: boolean,
    }): CancelablePromise<CreationResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/tester/problems/{id}/test-groups',
            path: {
                'id': id,
            },
            query: {
                'ordinal': ordinal,
                'name': name,
                'points': points,
                'is_sample': isSample,
            },
        });
    }
    /**
     * @returns GetTestGroupsResponseModel OK
     * @throws ApiError
     */
    public getTestGroups({
        id,
    }: {
        id: string,
    }): CancelablePromise<GetTestGroupsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/tester/problems/{id}/test-groups',
            path: {
                'id': id,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public updateTestGroup({
        id,
        groupId,
        requestBody,
    }: {
        id: string,
        groupId: string,
        requestBody: UpdateTestGroupRequestModel,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/tester/problems/{id}/test-groups/{group_id}',
            path: {
                'id': id,
                'group_id': groupId,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public deleteTestGroup({
        id,
        groupId,
    }: {
        id: string,
        groupId: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/tester/problems/{id}/test-groups/{group_id}',
            path: {
                'id': id,
                'group_id': groupId,
            },
        });
    }
    /**
     * @returns CreationResponseModel OK
     * @throws ApiError
     */
    public createProblemSample({
        id,
        ordinal,
        requestBody,
    }: {
        id: string,
        ordinal: number,
        requestBody: CreateSampleRequestModel,
    }): CancelablePromise<CreationResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/tester/problems/{id}/samples',
            path: {
                'id': id,
            },
            query: {
                'ordinal': ordinal,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * @returns GetSamplesResponseModel OK
     * @throws ApiError
     */
    public getProblemSamples({
        id,
    }: {
        id: string,
    }): CancelablePromise<GetSamplesResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/tester/problems/{id}/samples',
            path: {
                'id': id,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public deleteProblemSample({
        id,
        sampleId,
    }: {
        id: string,
        sampleId: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/tester/problems/{id}/samples/{sample_id}',
            path: {
                'id': id,
                'sample_id': sampleId,
            },
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
            url: '/tester/contests',
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
            url: '/tester/admin/contests',
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
            url: '/tester/user/{id}/contests',
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
            url: '/tester/workshop/contests',
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
            url: '/tester/public/contests',
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
            url: '/tester/contests/{contest_id}',
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
            url: '/tester/contests/{contest_id}',
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
            url: '/tester/contests/{contest_id}',
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
            url: '/tester/contests/{contest_id}/problems',
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
            url: '/tester/contests/{contest_id}/problems/{problem_id}',
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
            url: '/tester/contests/{contest_id}/problems/{problem_id}',
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
            url: '/tester/contests/{contest_id}/members',
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
            url: '/tester/contests/{contest_id}/members',
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
            url: '/tester/contests/{contest_id}/members',
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
            url: '/tester/contests/{contest_id}/members',
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
            url: '/tester/contests/{contest_id}/my-role',
            path: {
                'contest_id': contestId,
            },
        });
    }
    /**
     * @returns CreationResponseModel OK
     * @throws ApiError
     */
    public createAccessRequest({
        contestId,
    }: {
        contestId: string,
    }): CancelablePromise<CreationResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/tester/contests/{contest_id}/access-requests',
            path: {
                'contest_id': contestId,
            },
        });
    }
    /**
     * @returns ListAccessRequestsResponseModel OK
     * @throws ApiError
     */
    public listAccessRequests({
        contestId,
        page,
        pageSize,
        status,
    }: {
        contestId: string,
        page: number,
        pageSize: number,
        status?: 'pending' | 'approved' | 'rejected',
    }): CancelablePromise<ListAccessRequestsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/tester/contests/{contest_id}/access-requests',
            path: {
                'contest_id': contestId,
            },
            query: {
                'page': page,
                'pageSize': pageSize,
                'status': status,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public updateAccessRequest({
        contestId,
        userId,
        status,
    }: {
        contestId: string,
        userId: string,
        status: 'approved' | 'rejected',
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/tester/contests/{contest_id}/access-requests/{user_id}',
            path: {
                'contest_id': contestId,
                'user_id': userId,
            },
            query: {
                'status': status,
            },
        });
    }
    /**
     * @returns CreationResponseModel OK
     * @throws ApiError
     */
    public createInvitation({
        contestId,
        userId,
    }: {
        contestId: string,
        userId: string,
    }): CancelablePromise<CreationResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/tester/contests/{contest_id}/invitations',
            path: {
                'contest_id': contestId,
            },
            query: {
                'user_id': userId,
            },
        });
    }
    /**
     * @returns ListInvitationsResponseModel OK
     * @throws ApiError
     */
    public listInvitations({
        contestId,
        page,
        pageSize,
        status,
    }: {
        contestId: string,
        page: number,
        pageSize: number,
        status?: 'pending' | 'accepted' | 'declined' | 'revoked',
    }): CancelablePromise<ListInvitationsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/tester/contests/{contest_id}/invitations',
            path: {
                'contest_id': contestId,
            },
            query: {
                'page': page,
                'pageSize': pageSize,
                'status': status,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public revokeInvitation({
        contestId,
        invitationId,
    }: {
        contestId: string,
        invitationId: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/tester/contests/{contest_id}/invitations/{invitation_id}',
            path: {
                'contest_id': contestId,
                'invitation_id': invitationId,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public acceptInvitation({
        contestId,
        invitationId,
    }: {
        contestId: string,
        invitationId: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/tester/contests/{contest_id}/invitations/{invitation_id}/accept',
            path: {
                'contest_id': contestId,
                'invitation_id': invitationId,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public declineInvitation({
        contestId,
        invitationId,
    }: {
        contestId: string,
        invitationId: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/tester/contests/{contest_id}/invitations/{invitation_id}/decline',
            path: {
                'contest_id': contestId,
                'invitation_id': invitationId,
            },
        });
    }
    /**
     * @returns GetMonitorResponseModel OK
     * @throws ApiError
     */
    public getMonitor({
        contestId,
        page,
        pageSize,
    }: {
        contestId: string,
        page: number,
        pageSize: number,
    }): CancelablePromise<GetMonitorResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/tester/contests/{contest_id}/monitor',
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
     * @returns any OK
     * @throws ApiError
     */
    public updateProblemPosition({
        contestId,
        problemId,
        position,
    }: {
        contestId: string,
        problemId: string,
        position: number,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/tester/contests/{contest_id}/problems/{problem_id}/position',
            path: {
                'contest_id': contestId,
                'problem_id': problemId,
            },
            query: {
                'position': position,
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
            url: '/tester/contests/{contest_id}/submissions',
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
            url: '/tester/submissions',
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
            url: '/tester/submissions',
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
            url: '/tester/submissions/{submission_id}',
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
            url: '/tester/health',
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
            url: '/tester/users',
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
            url: '/tester/users/{id}',
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
            url: '/tester/users/{user_id}/submissions',
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
            url: '/tester/users/me',
        });
    }
}
