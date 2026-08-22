/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { AdminChangeEmailRequestModel } from '../models/AdminChangeEmailRequestModel';
import type { AdminSetPasswordRequestModel } from '../models/AdminSetPasswordRequestModel';
import type { AnswerContestClarificationRequestModel } from '../models/AnswerContestClarificationRequestModel';
import type { ApproveOrganizationJoinRequestModel } from '../models/ApproveOrganizationJoinRequestModel';
import type { AuthResponseModel } from '../models/AuthResponseModel';
import type { BatchCreateOrganizationUsersRequestModel } from '../models/BatchCreateOrganizationUsersRequestModel';
import type { BatchCreateOrganizationUsersResponseModel } from '../models/BatchCreateOrganizationUsersResponseModel';
import type { BlockProblemRequestModel } from '../models/BlockProblemRequestModel';
import type { BlockSubmissionRequestModel } from '../models/BlockSubmissionRequestModel';
import type { ChangePasswordRequestModel } from '../models/ChangePasswordRequestModel';
import type { ClaimTemporaryUserRequestModel } from '../models/ClaimTemporaryUserRequestModel';
import type { ClaimTemporaryUserResponseModel } from '../models/ClaimTemporaryUserResponseModel';
import type { CompileResult } from '../models/CompileResult';
import type { ConfirmEmailChangeRequestModel } from '../models/ConfirmEmailChangeRequestModel';
import type { ContestAnnouncementModel } from '../models/ContestAnnouncementModel';
import type { ContestClarificationModel } from '../models/ContestClarificationModel';
import type { ContestJoinRequestNullableResponseModel } from '../models/ContestJoinRequestNullableResponseModel';
import type { ContestJoinRequestResponseModel } from '../models/ContestJoinRequestResponseModel';
import type { CreateContestAnnouncementRequestModel } from '../models/CreateContestAnnouncementRequestModel';
import type { CreateContestClarificationRequestModel } from '../models/CreateContestClarificationRequestModel';
import type { CreateContestDraftRequestModel } from '../models/CreateContestDraftRequestModel';
import type { CreateContestJoinRequestModel } from '../models/CreateContestJoinRequestModel';
import type { CreatedPost } from '../models/CreatedPost';
import type { CreateOrganizationJoinRequestModel } from '../models/CreateOrganizationJoinRequestModel';
import type { CreateOrganizationResponseModel } from '../models/CreateOrganizationResponseModel';
import type { CreateSubmissionRequestModel } from '../models/CreateSubmissionRequestModel';
import type { CreationResponseModel } from '../models/CreationResponseModel';
import type { ForgotPasswordRequestModel } from '../models/ForgotPasswordRequestModel';
import type { GetContestProblemResponseModel } from '../models/GetContestProblemResponseModel';
import type { GetContestResponseModel } from '../models/GetContestResponseModel';
import type { GetHealthResponseModel } from '../models/GetHealthResponseModel';
import type { GetMyContestRoleResponseModel } from '../models/GetMyContestRoleResponseModel';
import type { GetOrganizationResponseModel } from '../models/GetOrganizationResponseModel';
import type { GetProblemResponseModel } from '../models/GetProblemResponseModel';
import type { GetSubmissionResponseModel } from '../models/GetSubmissionResponseModel';
import type { GetTeamResponseModel } from '../models/GetTeamResponseModel';
import type { GetUserDashboardResponseModel } from '../models/GetUserDashboardResponseModel';
import type { GetUserResponseModel } from '../models/GetUserResponseModel';
import type { InviteOrganizationMemberRequestModel } from '../models/InviteOrganizationMemberRequestModel';
import type { ListClaimedAccountsResponseModel } from '../models/ListClaimedAccountsResponseModel';
import type { ListContestAnnouncementsResponseModel } from '../models/ListContestAnnouncementsResponseModel';
import type { ListContestClarificationsResponseModel } from '../models/ListContestClarificationsResponseModel';
import type { ListContestDraftsResponseModel } from '../models/ListContestDraftsResponseModel';
import type { ListContestJoinRequestsResponseModel } from '../models/ListContestJoinRequestsResponseModel';
import type { ListContestMembersResponseModel } from '../models/ListContestMembersResponseModel';
import type { ListContestsResponseModel } from '../models/ListContestsResponseModel';
import type { ListContestTeamsResponseModel } from '../models/ListContestTeamsResponseModel';
import type { ListOrganizationInvitationsResponseModel } from '../models/ListOrganizationInvitationsResponseModel';
import type { ListOrganizationJoinRequestsResponseModel } from '../models/ListOrganizationJoinRequestsResponseModel';
import type { ListOrganizationMembersResponseModel } from '../models/ListOrganizationMembersResponseModel';
import type { ListOrganizationsResponseModel } from '../models/ListOrganizationsResponseModel';
import type { ListPostsResponseModel } from '../models/ListPostsResponseModel';
import type { ListProblemMembersResponseModel } from '../models/ListProblemMembersResponseModel';
import type { ListProblemsResponseModel } from '../models/ListProblemsResponseModel';
import type { ListProblemTeamsResponseModel } from '../models/ListProblemTeamsResponseModel';
import type { ListSubmissionsResponseModel } from '../models/ListSubmissionsResponseModel';
import type { ListTeamMembersResponseModel } from '../models/ListTeamMembersResponseModel';
import type { ListTeamsResponseModel } from '../models/ListTeamsResponseModel';
import type { ListUserContestsResponseModel } from '../models/ListUserContestsResponseModel';
import type { ListUsersResponseModel } from '../models/ListUsersResponseModel';
import type { LoginRequestModel } from '../models/LoginRequestModel';
import type { MessageResponse } from '../models/MessageResponse';
import type { NotificationsListResponseModel } from '../models/NotificationsListResponseModel';
import type { OrganizationInvitationModel } from '../models/OrganizationInvitationModel';
import type { OrganizationJoinRequestNullableResponseModel } from '../models/OrganizationJoinRequestNullableResponseModel';
import type { OrganizationJoinRequestResponseModel } from '../models/OrganizationJoinRequestResponseModel';
import type { PostModel } from '../models/PostModel';
import type { ProblemBlockStatusResponseModel } from '../models/ProblemBlockStatusResponseModel';
import type { ProblemLimits } from '../models/ProblemLimits';
import type { ProblemStatement } from '../models/ProblemStatement';
import type { ProblemTemplateModel } from '../models/ProblemTemplateModel';
import type { RegisterRequestModel } from '../models/RegisterRequestModel';
import type { ReorderContestProblemsRequestModel } from '../models/ReorderContestProblemsRequestModel';
import type { RequestEmailChangeRequestModel } from '../models/RequestEmailChangeRequestModel';
import type { ResendVerificationRequestModel } from '../models/ResendVerificationRequestModel';
import type { ResetPasswordRequestModel } from '../models/ResetPasswordRequestModel';
import type { ScoreboardResponseModel } from '../models/ScoreboardResponseModel';
import type { SupportedLanguagesResponse } from '../models/SupportedLanguagesResponse';
import type { TestReport } from '../models/TestReport';
import type { UnreadNotificationsCountResponseModel } from '../models/UnreadNotificationsCountResponseModel';
import type { UpdateContestRequestModel } from '../models/UpdateContestRequestModel';
import type { UpdateOrganizationRequestModel } from '../models/UpdateOrganizationRequestModel';
import type { UpdateProblemLimitsRequest } from '../models/UpdateProblemLimitsRequest';
import type { UpdateProblemRequestModel } from '../models/UpdateProblemRequestModel';
import type { UpdateProblemStatementRequest } from '../models/UpdateProblemStatementRequest';
import type { UpdateProblemTestsConfigRequest } from '../models/UpdateProblemTestsConfigRequest';
import type { UpdateTeamRequestModel } from '../models/UpdateTeamRequestModel';
import type { UpdateUserRequestModel } from '../models/UpdateUserRequestModel';
import type { ValidationReport } from '../models/ValidationReport';
import type { VerifyEmailRequestModel } from '../models/VerifyEmailRequestModel';
import type { WorkshopFileListResponse } from '../models/WorkshopFileListResponse';
import type { CancelablePromise } from '../core/CancelablePromise';
import type { BaseHttpRequest } from '../core/BaseHttpRequest';
export class DefaultService {
    constructor(public readonly httpRequest: BaseHttpRequest) {}
    /**
     * Get supported programming languages
     * @returns SupportedLanguagesResponse OK
     * @throws ApiError
     */
    public getLanguages(): CancelablePromise<SupportedLanguagesResponse> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/languages',
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
        organizationId,
        isTemplate,
    }: {
        page: number,
        pageSize: number,
        search?: string,
        descending?: boolean,
        owner?: boolean,
        organizationId?: string,
        isTemplate?: boolean,
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
                'is_template': isTemplate,
            },
        });
    }
    /**
     * @returns CreationResponseModel OK
     * @throws ApiError
     */
    public createProblem({
        title,
        templateId,
        organizationId,
    }: {
        title: string,
        templateId: string,
        organizationId?: string,
    }): CancelablePromise<CreationResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/problems',
            query: {
                'title': title,
                'organization_id': organizationId,
                'template_id': templateId,
            },
        });
    }
    /**
     * @returns ProblemTemplateModel OK
     * @throws ApiError
     */
    public listProblemTemplates({
        organizationId,
    }: {
        organizationId?: string,
    }): CancelablePromise<Array<ProblemTemplateModel>> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problem-templates',
            query: {
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
     * Change user email as admin (direct or with confirmation)
     * @returns any OK
     * @throws ApiError
     */
    public adminChangeEmail({
        username,
        requestBody,
    }: {
        username: string,
        requestBody: AdminChangeEmailRequestModel,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/admin/users/{username}/change-email',
            path: {
                'username': username,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * Set user password directly as admin
     * @returns any OK
     * @throws ApiError
     */
    public adminSetPassword({
        username,
        requestBody,
    }: {
        username: string,
        requestBody: AdminSetPasswordRequestModel,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/admin/users/{username}/set-password',
            path: {
                'username': username,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * Send password reset link to user email
     * @returns any OK
     * @throws ApiError
     */
    public adminSendPasswordReset({
        username,
    }: {
        username: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/admin/users/{username}/send-reset-password',
            path: {
                'username': username,
            },
        });
    }
    /**
     * Resend email verification link for user
     * @returns any OK
     * @throws ApiError
     */
    public adminResendVerification({
        username,
    }: {
        username: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/admin/users/{username}/resend-verification',
            path: {
                'username': username,
            },
        });
    }
    /**
     * @returns ListUserContestsResponseModel OK
     * @throws ApiError
     */
    public listUserContests({
        username,
        page,
        pageSize,
        search,
        sortBy,
        sortOrder,
    }: {
        username: string,
        page: number,
        pageSize: number,
        search?: string,
        sortBy?: 'last_submission_time' | 'created_at' | 'updated_at' | 'title',
        sortOrder?: 'asc' | 'desc',
    }): CancelablePromise<ListUserContestsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/users/{username}/contests',
            path: {
                'username': username,
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
     * @returns CreationResponseModel OK
     * @throws ApiError
     */
    public createContest({
        orgLogin,
        title,
        login,
    }: {
        orgLogin: string,
        title: string,
        login?: string,
    }): CancelablePromise<CreationResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations/{org_login}/contests',
            path: {
                'org_login': orgLogin,
            },
            query: {
                'title': title,
                'login': login,
            },
        });
    }
    /**
     * @returns ListContestsResponseModel OK
     * @throws ApiError
     */
    public listOrganizationContests({
        orgLogin,
        page,
        pageSize,
        search,
    }: {
        orgLogin: string,
        page: number,
        pageSize: number,
        search?: string,
    }): CancelablePromise<ListContestsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/organizations/{org_login}/contests',
            path: {
                'org_login': orgLogin,
            },
            query: {
                'page': page,
                'pageSize': pageSize,
                'search': search,
            },
        });
    }
    /**
     * @returns GetContestResponseModel OK
     * @throws ApiError
     */
    public getContest({
        orgLogin,
        contestLogin,
    }: {
        orgLogin: string,
        contestLogin: string,
    }): CancelablePromise<GetContestResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/organizations/{org_login}/contests/{contest_login}',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public deleteContest({
        orgLogin,
        contestLogin,
    }: {
        orgLogin: string,
        contestLogin: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/organizations/{org_login}/contests/{contest_login}',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public updateContest({
        orgLogin,
        contestLogin,
        requestBody,
    }: {
        orgLogin: string,
        contestLogin: string,
        requestBody: UpdateContestRequestModel,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/organizations/{org_login}/contests/{contest_login}',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * Download PDF booklet with contest statements
     * @returns binary PDF booklet containing contest problem statements
     * @throws ApiError
     */
    public downloadContestStatementsPdf({
        orgLogin,
        contestLogin,
        lang = 'ru',
    }: {
        orgLogin: string,
        contestLogin: string,
        lang?: string,
    }): CancelablePromise<Blob> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/organizations/{org_login}/contests/{contest_login}/statements.pdf',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
            },
            query: {
                'lang': lang,
            },
        });
    }
    /**
     * @returns CreationResponseModel OK
     * @throws ApiError
     */
    public createContestProblem({
        orgLogin,
        contestLogin,
        problemId,
        packageId,
    }: {
        orgLogin: string,
        contestLogin: string,
        problemId: string,
        packageId?: string,
    }): CancelablePromise<CreationResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations/{org_login}/contests/{contest_login}/problems',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
            },
            query: {
                'problem_id': problemId,
                'package_id': packageId,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public reorderContestProblems({
        orgLogin,
        contestLogin,
        requestBody,
    }: {
        orgLogin: string,
        contestLogin: string,
        requestBody: ReorderContestProblemsRequestModel,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'PUT',
            url: '/organizations/{org_login}/contests/{contest_login}/problems/reorder',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * @returns GetContestProblemResponseModel OK
     * @throws ApiError
     */
    public getContestProblem({
        orgLogin,
        contestLogin,
        problemId,
    }: {
        orgLogin: string,
        contestLogin: string,
        problemId: string,
    }): CancelablePromise<GetContestProblemResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/organizations/{org_login}/contests/{contest_login}/problems/{problem_id}',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
                'problem_id': problemId,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public deleteContestProblem({
        orgLogin,
        contestLogin,
        problemId,
    }: {
        orgLogin: string,
        contestLogin: string,
        problemId: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/organizations/{org_login}/contests/{contest_login}/problems/{problem_id}',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
                'problem_id': problemId,
            },
        });
    }
    /**
     * @returns ListContestMembersResponseModel OK
     * @throws ApiError
     */
    public listContestMembers({
        orgLogin,
        contestLogin,
        page,
        pageSize,
    }: {
        orgLogin: string,
        contestLogin: string,
        page: number,
        pageSize: number,
    }): CancelablePromise<ListContestMembersResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/organizations/{org_login}/contests/{contest_login}/members',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
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
        orgLogin,
        contestLogin,
        userId,
    }: {
        orgLogin: string,
        contestLogin: string,
        userId: string,
    }): CancelablePromise<CreationResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations/{org_login}/contests/{contest_login}/members',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
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
        orgLogin,
        contestLogin,
    }: {
        userId: string,
        orgLogin: string,
        contestLogin: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/organizations/{org_login}/contests/{contest_login}/members',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
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
        orgLogin,
        contestLogin,
        userId,
        role,
    }: {
        orgLogin: string,
        contestLogin: string,
        userId: string,
        role: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/organizations/{org_login}/contests/{contest_login}/members',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
            },
            query: {
                'user_id': userId,
                'role': role,
            },
        });
    }
    /**
     * @returns ListContestTeamsResponseModel OK
     * @throws ApiError
     */
    public listContestTeams({
        orgLogin,
        contestLogin,
    }: {
        orgLogin: string,
        contestLogin: string,
    }): CancelablePromise<ListContestTeamsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/organizations/{org_login}/contests/{contest_login}/teams',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public createContestTeam({
        orgLogin,
        contestLogin,
        teamId,
        role = 'participant',
    }: {
        orgLogin: string,
        contestLogin: string,
        teamId: string,
        role?: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations/{org_login}/contests/{contest_login}/teams',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
            },
            query: {
                'team_id': teamId,
                'role': role,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public deleteContestTeam({
        orgLogin,
        contestLogin,
        teamId,
    }: {
        orgLogin: string,
        contestLogin: string,
        teamId: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/organizations/{org_login}/contests/{contest_login}/teams',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
            },
            query: {
                'team_id': teamId,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public updateContestTeam({
        orgLogin,
        contestLogin,
        teamId,
        role,
    }: {
        orgLogin: string,
        contestLogin: string,
        teamId: string,
        role: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/organizations/{org_login}/contests/{contest_login}/teams',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
            },
            query: {
                'team_id': teamId,
                'role': role,
            },
        });
    }
    /**
     * @returns GetMyContestRoleResponseModel OK
     * @throws ApiError
     */
    public getMyContestRole({
        orgLogin,
        contestLogin,
    }: {
        orgLogin: string,
        contestLogin: string,
    }): CancelablePromise<GetMyContestRoleResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/organizations/{org_login}/contests/{contest_login}/my-role',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
            },
        });
    }
    /**
     * @returns ListSubmissionsResponseModel OK
     * @throws ApiError
     */
    public listContestSubmissions({
        orgLogin,
        contestLogin,
        page,
        pageSize,
        userId,
        problemId,
        state,
        sortOrder,
        language,
    }: {
        orgLogin: string,
        contestLogin: string,
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
            url: '/organizations/{org_login}/contests/{contest_login}/submissions',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
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
     * @returns any OK
     * @throws ApiError
     */
    public rejudgeSubmission({
        orgLogin,
        contestLogin,
        submissionId,
    }: {
        orgLogin: string,
        contestLogin: string,
        submissionId: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations/{org_login}/contests/{contest_login}/submissions/{submission_id}/rejudge',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
                'submission_id': submissionId,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public blockSubmission({
        orgLogin,
        contestLogin,
        submissionId,
        requestBody,
    }: {
        orgLogin: string,
        contestLogin: string,
        submissionId: string,
        requestBody?: BlockSubmissionRequestModel,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations/{org_login}/contests/{contest_login}/submissions/{submission_id}/block',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
                'submission_id': submissionId,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public unblockSubmission({
        orgLogin,
        contestLogin,
        submissionId,
    }: {
        orgLogin: string,
        contestLogin: string,
        submissionId: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations/{org_login}/contests/{contest_login}/submissions/{submission_id}/unblock',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
                'submission_id': submissionId,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public blockProblemForUser({
        orgLogin,
        contestLogin,
        userId,
        problemId,
        requestBody,
    }: {
        orgLogin: string,
        contestLogin: string,
        userId: string,
        problemId: string,
        requestBody?: BlockProblemRequestModel,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations/{org_login}/contests/{contest_login}/participants/{user_id}/problems/{problem_id}/block',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
                'user_id': userId,
                'problem_id': problemId,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public unblockProblemForUser({
        orgLogin,
        contestLogin,
        userId,
        problemId,
        rejudgeSubmissions = false,
    }: {
        orgLogin: string,
        contestLogin: string,
        userId: string,
        problemId: string,
        rejudgeSubmissions?: boolean,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/organizations/{org_login}/contests/{contest_login}/participants/{user_id}/problems/{problem_id}/block',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
                'user_id': userId,
                'problem_id': problemId,
            },
            query: {
                'rejudge_submissions': rejudgeSubmissions,
            },
        });
    }
    /**
     * @returns ProblemBlockStatusResponseModel OK
     * @throws ApiError
     */
    public getProblemBlockStatusForUser({
        orgLogin,
        contestLogin,
        userId,
        problemId,
    }: {
        orgLogin: string,
        contestLogin: string,
        userId: string,
        problemId: string,
    }): CancelablePromise<ProblemBlockStatusResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/organizations/{org_login}/contests/{contest_login}/participants/{user_id}/problems/{problem_id}/block',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
                'user_id': userId,
                'problem_id': problemId,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public rejudgeContestProblem({
        orgLogin,
        contestLogin,
        problemId,
    }: {
        orgLogin: string,
        contestLogin: string,
        problemId: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations/{org_login}/contests/{contest_login}/problems/{problem_id}/rejudge',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
                'problem_id': problemId,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public rejudgeContest({
        orgLogin,
        contestLogin,
    }: {
        orgLogin: string,
        contestLogin: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations/{org_login}/contests/{contest_login}/rejudge',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
            },
        });
    }
    /**
     * @returns ScoreboardResponseModel OK
     * @throws ApiError
     */
    public getContestScoreboard({
        orgLogin,
        contestLogin,
        unfrozen,
    }: {
        orgLogin: string,
        contestLogin: string,
        /**
         * Whether to return unfrozen scoreboard (managers only)
         */
        unfrozen?: boolean,
    }): CancelablePromise<ScoreboardResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/organizations/{org_login}/contests/{contest_login}/scoreboard',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
            },
            query: {
                'unfrozen': unfrozen,
            },
        });
    }
    /**
     * @returns CreationResponseModel OK
     * @throws ApiError
     */
    public createContestDraft({
        orgLogin,
        contestLogin,
        requestBody,
    }: {
        orgLogin: string,
        contestLogin: string,
        requestBody: CreateContestDraftRequestModel,
    }): CancelablePromise<CreationResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations/{org_login}/contests/{contest_login}/drafts',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * @returns ListContestDraftsResponseModel OK
     * @throws ApiError
     */
    public listContestDrafts({
        orgLogin,
        contestLogin,
        page,
        pageSize,
    }: {
        orgLogin: string,
        contestLogin: string,
        page?: number,
        pageSize?: number,
    }): CancelablePromise<ListContestDraftsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/organizations/{org_login}/contests/{contest_login}/drafts',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
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
    public deleteContestDraft({
        orgLogin,
        contestLogin,
        draftId,
    }: {
        orgLogin: string,
        contestLogin: string,
        draftId: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/organizations/{org_login}/contests/{contest_login}/drafts/{draft_id}',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
                'draft_id': draftId,
            },
        });
    }
    /**
     * List pending contest join requests
     * @returns ListContestJoinRequestsResponseModel List of contest join requests
     * @throws ApiError
     */
    public listContestJoinRequests({
        orgLogin,
        contestLogin,
    }: {
        orgLogin: string,
        contestLogin: string,
    }): CancelablePromise<ListContestJoinRequestsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/organizations/{org_login}/contests/{contest_login}/requests',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
            },
        });
    }
    /**
     * Request to participate in contest
     * @returns ContestJoinRequestResponseModel Request created or user registered
     * @throws ApiError
     */
    public createContestJoinRequest({
        orgLogin,
        contestLogin,
        requestBody,
    }: {
        orgLogin: string,
        contestLogin: string,
        requestBody?: CreateContestJoinRequestModel,
    }): CancelablePromise<ContestJoinRequestResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations/{org_login}/contests/{contest_login}/requests',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * Get my pending contest join request
     * @returns ContestJoinRequestNullableResponseModel My pending contest join request
     * @throws ApiError
     */
    public getMyContestJoinRequest({
        orgLogin,
        contestLogin,
    }: {
        orgLogin: string,
        contestLogin: string,
    }): CancelablePromise<ContestJoinRequestNullableResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/organizations/{org_login}/contests/{contest_login}/requests/mine',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
            },
        });
    }
    /**
     * Cancel my contest join request
     * @returns any Request canceled
     * @throws ApiError
     */
    public cancelContestJoinRequest({
        orgLogin,
        contestLogin,
    }: {
        orgLogin: string,
        contestLogin: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/organizations/{org_login}/contests/{contest_login}/requests/mine',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
            },
        });
    }
    /**
     * Approve contest join request
     * @returns any Request approved
     * @throws ApiError
     */
    public approveContestJoinRequest({
        orgLogin,
        contestLogin,
        id,
    }: {
        orgLogin: string,
        contestLogin: string,
        id: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations/{org_login}/contests/{contest_login}/requests/{id}/approve',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
                'id': id,
            },
        });
    }
    /**
     * Reject contest join request
     * @returns any Request rejected
     * @throws ApiError
     */
    public rejectContestJoinRequest({
        orgLogin,
        contestLogin,
        id,
    }: {
        orgLogin: string,
        contestLogin: string,
        id: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations/{org_login}/contests/{contest_login}/requests/{id}/reject',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
                'id': id,
            },
        });
    }
    /**
     * List contest announcements
     * @returns ListContestAnnouncementsResponseModel List of contest announcements
     * @throws ApiError
     */
    public listContestAnnouncements({
        orgLogin,
        contestLogin,
        page = 1,
        pageSize = 50,
    }: {
        orgLogin: string,
        contestLogin: string,
        page?: number,
        pageSize?: number,
    }): CancelablePromise<ListContestAnnouncementsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/organizations/{org_login}/contests/{contest_login}/announcements',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
            },
            query: {
                'page': page,
                'pageSize': pageSize,
            },
        });
    }
    /**
     * Create a contest announcement (jury only)
     * @returns ContestAnnouncementModel Announcement created
     * @throws ApiError
     */
    public createContestAnnouncement({
        orgLogin,
        contestLogin,
        requestBody,
    }: {
        orgLogin: string,
        contestLogin: string,
        requestBody: CreateContestAnnouncementRequestModel,
    }): CancelablePromise<ContestAnnouncementModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations/{org_login}/contests/{contest_login}/announcements',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * Delete a contest announcement (jury only)
     * @returns any Announcement deleted
     * @throws ApiError
     */
    public deleteContestAnnouncement({
        orgLogin,
        contestLogin,
        announcementId,
    }: {
        orgLogin: string,
        contestLogin: string,
        announcementId: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/organizations/{org_login}/contests/{contest_login}/announcements/{announcement_id}',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
                'announcement_id': announcementId,
            },
        });
    }
    /**
     * List contest clarifications (for participant: own questions; for jury: all questions)
     * @returns ListContestClarificationsResponseModel List of contest clarifications
     * @throws ApiError
     */
    public listContestClarifications({
        orgLogin,
        contestLogin,
        problemId,
        status,
        page = 1,
        pageSize = 50,
    }: {
        orgLogin: string,
        contestLogin: string,
        problemId?: string,
        status?: string,
        page?: number,
        pageSize?: number,
    }): CancelablePromise<ListContestClarificationsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/organizations/{org_login}/contests/{contest_login}/clarifications',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
            },
            query: {
                'problem_id': problemId,
                'status': status,
                'page': page,
                'pageSize': pageSize,
            },
        });
    }
    /**
     * Submit a clarification question to the jury
     * @returns ContestClarificationModel Clarification question submitted
     * @throws ApiError
     */
    public createContestClarification({
        orgLogin,
        contestLogin,
        requestBody,
    }: {
        orgLogin: string,
        contestLogin: string,
        requestBody: CreateContestClarificationRequestModel,
    }): CancelablePromise<ContestClarificationModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations/{org_login}/contests/{contest_login}/clarifications',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * Answer a clarification question (jury only)
     * @returns ContestClarificationModel Clarification answered
     * @throws ApiError
     */
    public answerContestClarification({
        orgLogin,
        contestLogin,
        clarificationId,
        requestBody,
    }: {
        orgLogin: string,
        contestLogin: string,
        clarificationId: string,
        requestBody: AnswerContestClarificationRequestModel,
    }): CancelablePromise<ContestClarificationModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations/{org_login}/contests/{contest_login}/clarifications/{clarification_id}/answer',
            path: {
                'org_login': orgLogin,
                'contest_login': contestLogin,
                'clarification_id': clarificationId,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * @returns CreationResponseModel OK
     * @throws ApiError
     */
    public createSubmission({
        problemId,
        organizationLogin,
        contestLogin,
        language,
        requestBody,
    }: {
        problemId: string,
        organizationLogin: string,
        contestLogin: string,
        language: number,
        requestBody: CreateSubmissionRequestModel,
    }): CancelablePromise<CreationResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/submissions',
            query: {
                'problem_id': problemId,
                'organization_login': organizationLogin,
                'contest_login': contestLogin,
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
        username,
    }: {
        username: string,
    }): CancelablePromise<GetUserResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/users/{username}',
            path: {
                'username': username,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public updateUser({
        username,
        requestBody,
    }: {
        username: string,
        requestBody: UpdateUserRequestModel,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/users/{username}',
            path: {
                'username': username,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * @returns ListSubmissionsResponseModel OK
     * @throws ApiError
     */
    public listUserSubmissions({
        username,
        page,
        pageSize,
        contestId,
        problemId,
        state,
        sortOrder,
    }: {
        username: string,
        page: number,
        pageSize: number,
        contestId?: string,
        problemId?: string,
        state?: number,
        sortOrder?: 'asc' | 'desc',
    }): CancelablePromise<ListSubmissionsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/users/{username}/submissions',
            path: {
                'username': username,
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
     * @returns GetUserDashboardResponseModel OK
     * @throws ApiError
     */
    public getMyDashboard(): CancelablePromise<GetUserDashboardResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/users/me/dashboard',
        });
    }
    /**
     * Claim temporary user results into permanent account
     * @returns ClaimTemporaryUserResponseModel OK
     * @throws ApiError
     */
    public claimTemporaryUser({
        requestBody,
    }: {
        requestBody: ClaimTemporaryUserRequestModel,
    }): CancelablePromise<ClaimTemporaryUserResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/users/claim-temporary',
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * List claimed temporary accounts
     * @returns ListClaimedAccountsResponseModel OK
     * @throws ApiError
     */
    public listMyClaimedAccounts(): CancelablePromise<ListClaimedAccountsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/users/me/claimed-accounts',
        });
    }
    /**
     * Change password for current user
     * @returns AuthResponseModel Password successfully changed
     * @throws ApiError
     */
    public changePassword({
        requestBody,
    }: {
        requestBody: ChangePasswordRequestModel,
    }): CancelablePromise<AuthResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/users/me/change-password',
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * Request email change for current user
     * @returns any Email change requested
     * @throws ApiError
     */
    public requestEmailChange({
        requestBody,
    }: {
        requestBody: RequestEmailChangeRequestModel,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/users/me/request-email-change',
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * Get user avatar by username
     * @returns string User avatar
     * @throws ApiError
     */
    public getUserAvatar({
        username,
        ifNoneMatch,
    }: {
        username: string,
        ifNoneMatch?: string,
    }): CancelablePromise<string> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/users/{username}/avatar',
            path: {
                'username': username,
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
        username,
        formData,
    }: {
        username: string,
        formData: {
            avatar?: Blob;
        },
    }): CancelablePromise<{
        imgId?: string;
    }> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/users/{username}/avatar',
            path: {
                'username': username,
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
        username,
    }: {
        username: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/users/{username}/avatar',
            path: {
                'username': username,
            },
        });
    }
    /**
     * @returns ListProblemTeamsResponseModel OK
     * @throws ApiError
     */
    public listProblemTeams({
        id,
    }: {
        id: string,
    }): CancelablePromise<ListProblemTeamsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{id}/teams',
            path: {
                'id': id,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public createProblemTeam({
        id,
        teamId,
        permission = 'read',
    }: {
        id: string,
        teamId: string,
        permission?: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/problems/{id}/teams',
            path: {
                'id': id,
            },
            query: {
                'team_id': teamId,
                'permission': permission,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public deleteProblemTeam({
        id,
        teamId,
    }: {
        id: string,
        teamId: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/problems/{id}/teams',
            path: {
                'id': id,
            },
            query: {
                'team_id': teamId,
            },
        });
    }
    /**
     * @returns any OK
     * @throws ApiError
     */
    public updateProblemTeam({
        id,
        teamId,
        permission,
    }: {
        id: string,
        teamId: string,
        permission: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/problems/{id}/teams',
            path: {
                'id': id,
            },
            query: {
                'team_id': teamId,
                'permission': permission,
            },
        });
    }
    /**
     * @returns ListProblemMembersResponseModel OK
     * @throws ApiError
     */
    public listProblemMembers({
        id,
        page = 1,
        pageSize = 10,
    }: {
        id: string,
        page?: number,
        pageSize?: number,
    }): CancelablePromise<ListProblemMembersResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{id}/members',
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
     * @returns any OK
     * @throws ApiError
     */
    public createProblemMember({
        id,
        userId,
        role = 'viewer',
    }: {
        id: string,
        userId: string,
        role?: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/problems/{id}/members',
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
     * @returns any OK
     * @throws ApiError
     */
    public deleteProblemMember({
        id,
        userId,
    }: {
        id: string,
        userId: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/problems/{id}/members',
            path: {
                'id': id,
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
    public updateProblemMember({
        id,
        userId,
        role,
    }: {
        id: string,
        userId: string,
        role: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/problems/{id}/members',
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
        lang,
    }: {
        problemId: string,
        lang?: string,
    }): CancelablePromise<ProblemStatement> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{problemId}/statement',
            path: {
                'problemId': problemId,
            },
            query: {
                'lang': lang,
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
        lang,
    }: {
        problemId: string,
        requestBody: UpdateProblemStatementRequest,
        lang?: string,
    }): CancelablePromise<ProblemStatement> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/problems/{problemId}/statement',
            path: {
                'problemId': problemId,
            },
            query: {
                'lang': lang,
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
        requestBody: string,
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
            mediaType: 'text/plain',
        });
    }
    /**
     * Get checker file content
     * @returns string Checker content
     * @throws ApiError
     */
    public getProblemChecker({
        problemId,
        name,
    }: {
        problemId: string,
        name: string,
    }): CancelablePromise<string> {
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
        requestBody: string,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'PUT',
            url: '/problems/{problemId}/checkers/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
            body: requestBody,
            mediaType: 'text/plain',
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
        requestBody: string,
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
            mediaType: 'text/plain',
        });
    }
    /**
     * Get generator file content
     * @returns string Generator content
     * @throws ApiError
     */
    public getProblemGenerator({
        problemId,
        name,
    }: {
        problemId: string,
        name: string,
    }): CancelablePromise<string> {
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
        requestBody: string,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'PUT',
            url: '/problems/{problemId}/generators/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
            body: requestBody,
            mediaType: 'text/plain',
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
        requestBody: string,
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
            mediaType: 'text/plain',
        });
    }
    /**
     * Get interactor file content
     * @returns string Interactor content
     * @throws ApiError
     */
    public getProblemInteractor({
        problemId,
        name,
    }: {
        problemId: string,
        name: string,
    }): CancelablePromise<string> {
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
        requestBody: string,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'PUT',
            url: '/problems/{problemId}/interactors/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
            body: requestBody,
            mediaType: 'text/plain',
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
     * List library files
     * @returns WorkshopFileListResponse List of library files
     * @throws ApiError
     */
    public listProblemLibs({
        problemId,
    }: {
        problemId: string,
    }): CancelablePromise<WorkshopFileListResponse> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{problemId}/lib',
            path: {
                'problemId': problemId,
            },
        });
    }
    /**
     * Create library file
     * @returns MessageResponse Library file created successfully
     * @throws ApiError
     */
    public createProblemLib({
        problemId,
        name,
        requestBody,
    }: {
        problemId: string,
        name: string,
        requestBody: string,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/problems/{problemId}/lib',
            path: {
                'problemId': problemId,
            },
            query: {
                'name': name,
            },
            body: requestBody,
            mediaType: 'text/plain',
        });
    }
    /**
     * Get library file content
     * @returns string Library file content
     * @throws ApiError
     */
    public getProblemLib({
        problemId,
        name,
    }: {
        problemId: string,
        name: string,
    }): CancelablePromise<string> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/problems/{problemId}/lib/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
        });
    }
    /**
     * Update library file
     * @returns MessageResponse Library file updated successfully
     * @throws ApiError
     */
    public updateProblemLib({
        problemId,
        name,
        requestBody,
    }: {
        problemId: string,
        name: string,
        requestBody: string,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'PUT',
            url: '/problems/{problemId}/lib/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
            body: requestBody,
            mediaType: 'text/plain',
        });
    }
    /**
     * Delete library file
     * @returns MessageResponse Library file deleted successfully
     * @throws ApiError
     */
    public deleteProblemLib({
        problemId,
        name,
    }: {
        problemId: string,
        name: string,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/problems/{problemId}/lib/{name}',
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
        requestBody: string,
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
            mediaType: 'text/plain',
        });
    }
    /**
     * Get author solution file content
     * @returns string Author solution file content
     * @throws ApiError
     */
    public getProblemWorkshopSubmission({
        problemId,
        name,
    }: {
        problemId: string,
        name: string,
    }): CancelablePromise<string> {
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
        requestBody: string,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'PUT',
            url: '/problems/{problemId}/submissions/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
            body: requestBody,
            mediaType: 'text/plain',
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
        requestBody: string,
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
            mediaType: 'text/plain',
        });
    }
    /**
     * Get validator file content
     * @returns string Validator content
     * @throws ApiError
     */
    public getProblemValidator({
        problemId,
        name,
    }: {
        problemId: string,
        name: string,
    }): CancelablePromise<string> {
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
        requestBody: string,
    }): CancelablePromise<MessageResponse> {
        return this.httpRequest.request({
            method: 'PUT',
            url: '/problems/{problemId}/validators/{name}',
            path: {
                'problemId': problemId,
                'name': name,
            },
            body: requestBody,
            mediaType: 'text/plain',
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
     * @returns CreateOrganizationResponseModel Organization created successfully
     * @throws ApiError
     */
    public createOrganization({
        name,
        login,
        joinPolicy,
    }: {
        name: string,
        login?: string,
        joinPolicy?: 'open' | 'by_request' | 'invite_only',
    }): CancelablePromise<CreateOrganizationResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations',
            query: {
                'name': name,
                'login': login,
                'join_policy': joinPolicy,
            },
        });
    }
    /**
     * Get organization by login
     * @returns GetOrganizationResponseModel Organization details
     * @throws ApiError
     */
    public getOrganization({
        login,
    }: {
        login: string,
    }): CancelablePromise<GetOrganizationResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/organizations/{login}',
            path: {
                'login': login,
            },
        });
    }
    /**
     * Update organization
     * @returns any Organization updated successfully
     * @throws ApiError
     */
    public updateOrganization({
        login,
        requestBody,
    }: {
        login: string,
        requestBody: UpdateOrganizationRequestModel,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/organizations/{login}',
            path: {
                'login': login,
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
        login,
    }: {
        login: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/organizations/{login}',
            path: {
                'login': login,
            },
        });
    }
    /**
     * List organization members
     * @returns ListOrganizationMembersResponseModel List of organization members
     * @throws ApiError
     */
    public listOrganizationMembers({
        login,
        page,
        pageSize,
    }: {
        login: string,
        page: number,
        pageSize: number,
    }): CancelablePromise<ListOrganizationMembersResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/organizations/{login}/members',
            path: {
                'login': login,
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
        login,
        userId,
        role,
    }: {
        login: string,
        userId: string,
        role: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations/{login}/members',
            path: {
                'login': login,
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
        login,
        userId,
    }: {
        login: string,
        userId: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/organizations/{login}/members',
            path: {
                'login': login,
            },
            query: {
                'user_id': userId,
            },
        });
    }
    /**
     * Batch create users in organization
     * @returns BatchCreateOrganizationUsersResponseModel OK
     * @throws ApiError
     */
    public batchCreateOrganizationUsers({
        login,
        requestBody,
    }: {
        login: string,
        requestBody: BatchCreateOrganizationUsersRequestModel,
    }): CancelablePromise<BatchCreateOrganizationUsersResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations/{login}/members/batch',
            path: {
                'login': login,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * List pending organization invitations
     * @returns ListOrganizationInvitationsResponseModel List of invitations
     * @throws ApiError
     */
    public listOrganizationInvitations({
        login,
    }: {
        login: string,
    }): CancelablePromise<ListOrganizationInvitationsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/organizations/{login}/invitations',
            path: {
                'login': login,
            },
        });
    }
    /**
     * Invite user to organization
     * @returns OrganizationInvitationModel Invitation sent
     * @throws ApiError
     */
    public inviteOrganizationMember({
        login,
        requestBody,
    }: {
        login: string,
        requestBody: InviteOrganizationMemberRequestModel,
    }): CancelablePromise<OrganizationInvitationModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations/{login}/invitations',
            path: {
                'login': login,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * Cancel organization invitation
     * @returns any Invitation canceled
     * @throws ApiError
     */
    public cancelOrganizationInvitation({
        login,
        id,
    }: {
        login: string,
        id: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/organizations/{login}/invitations/{id}',
            path: {
                'login': login,
                'id': id,
            },
        });
    }
    /**
     * Accept organization invitation
     * @returns any Invitation accepted
     * @throws ApiError
     */
    public acceptOrganizationInvitation({
        id,
    }: {
        id: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/invitations/{id}/accept',
            path: {
                'id': id,
            },
        });
    }
    /**
     * Decline organization invitation
     * @returns any Invitation declined
     * @throws ApiError
     */
    public declineOrganizationInvitation({
        id,
    }: {
        id: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/invitations/{id}/decline',
            path: {
                'id': id,
            },
        });
    }
    /**
     * List pending organization join requests
     * @returns ListOrganizationJoinRequestsResponseModel List of join requests
     * @throws ApiError
     */
    public listOrganizationJoinRequests({
        login,
    }: {
        login: string,
    }): CancelablePromise<ListOrganizationJoinRequestsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/organizations/{login}/requests',
            path: {
                'login': login,
            },
        });
    }
    /**
     * Request to join organization
     * @returns OrganizationJoinRequestResponseModel Request submitted or user joined
     * @throws ApiError
     */
    public createOrganizationJoinRequest({
        login,
        requestBody,
    }: {
        login: string,
        requestBody?: CreateOrganizationJoinRequestModel,
    }): CancelablePromise<OrganizationJoinRequestResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations/{login}/requests',
            path: {
                'login': login,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * Get my pending join request to organization
     * @returns OrganizationJoinRequestNullableResponseModel My pending join request
     * @throws ApiError
     */
    public getMyOrganizationJoinRequest({
        login,
    }: {
        login: string,
    }): CancelablePromise<OrganizationJoinRequestNullableResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/organizations/{login}/requests/mine',
            path: {
                'login': login,
            },
        });
    }
    /**
     * Cancel my join request to organization
     * @returns any Request canceled
     * @throws ApiError
     */
    public cancelOrganizationJoinRequest({
        login,
    }: {
        login: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/organizations/{login}/requests/mine',
            path: {
                'login': login,
            },
        });
    }
    /**
     * Approve organization join request
     * @returns any Request approved
     * @throws ApiError
     */
    public approveOrganizationJoinRequest({
        login,
        id,
        requestBody,
    }: {
        login: string,
        id: string,
        requestBody?: ApproveOrganizationJoinRequestModel,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations/{login}/requests/{id}/approve',
            path: {
                'login': login,
                'id': id,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * Reject organization join request
     * @returns any Request rejected
     * @throws ApiError
     */
    public rejectOrganizationJoinRequest({
        login,
        id,
    }: {
        login: string,
        id: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/organizations/{login}/requests/{id}/reject',
            path: {
                'login': login,
                'id': id,
            },
        });
    }
    /**
     * List user notifications
     * @returns NotificationsListResponseModel List of notifications
     * @throws ApiError
     */
    public listNotifications({
        page = 1,
        pageSize = 20,
        unreadOnly,
    }: {
        page?: number,
        pageSize?: number,
        unreadOnly?: boolean,
    }): CancelablePromise<NotificationsListResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/notifications',
            query: {
                'page': page,
                'pageSize': pageSize,
                'unread_only': unreadOnly,
            },
        });
    }
    /**
     * Get count of unread notifications
     * @returns UnreadNotificationsCountResponseModel Unread notifications count
     * @throws ApiError
     */
    public getUnreadNotificationsCount(): CancelablePromise<UnreadNotificationsCountResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/notifications/unread-count',
        });
    }
    /**
     * Mark a notification as read
     * @returns any Notification marked as read
     * @throws ApiError
     */
    public markNotificationAsRead({
        id,
    }: {
        id: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/notifications/{id}/read',
            path: {
                'id': id,
            },
        });
    }
    /**
     * Mark all notifications as read
     * @returns any All notifications marked as read
     * @throws ApiError
     */
    public markAllNotificationsAsRead(): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/notifications/read-all',
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
        role = 'member',
    }: {
        id: string,
        userId: string,
        role?: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/teams/{id}/members',
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
     * Update member role in team
     * @returns any Member role updated successfully
     * @throws ApiError
     */
    public updateTeamMemberRole({
        id,
        userId,
        role,
    }: {
        id: string,
        userId: string,
        role: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/teams/{id}/members',
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
     * List contests accessible by team
     * @returns ListContestsResponseModel OK
     * @throws ApiError
     */
    public listTeamContests({
        id,
        page = 1,
        pageSize = 10,
    }: {
        id: string,
        page?: number,
        pageSize?: number,
    }): CancelablePromise<ListContestsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/teams/{id}/contests',
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
     * List problems accessible by team
     * @returns ListProblemsResponseModel OK
     * @throws ApiError
     */
    public listTeamProblems({
        id,
        page = 1,
        pageSize = 10,
    }: {
        id: string,
        page?: number,
        pageSize?: number,
    }): CancelablePromise<ListProblemsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/teams/{id}/problems',
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
    /**
     * @returns AuthResponseModel Successfully registered
     * @throws ApiError
     */
    public register({
        requestBody,
    }: {
        requestBody: RegisterRequestModel,
    }): CancelablePromise<AuthResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/auth/register',
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * @returns AuthResponseModel Successfully logged in
     * @throws ApiError
     */
    public login({
        requestBody,
    }: {
        requestBody: LoginRequestModel,
    }): CancelablePromise<AuthResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/auth/login',
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * @returns any Successfully logged out
     * @throws ApiError
     */
    public logout(): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/auth/logout',
        });
    }
    /**
     * @returns AuthResponseModel Successfully verified email
     * @throws ApiError
     */
    public verifyEmail({
        requestBody,
    }: {
        requestBody: VerifyEmailRequestModel,
    }): CancelablePromise<AuthResponseModel> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/auth/verify-email',
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * @returns any Verification email resent
     * @throws ApiError
     */
    public resendVerification({
        requestBody,
    }: {
        requestBody: ResendVerificationRequestModel,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/auth/resend-verification',
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * @returns any Password reset email sent
     * @throws ApiError
     */
    public forgotPassword({
        requestBody,
    }: {
        requestBody: ForgotPasswordRequestModel,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/auth/forgot-password',
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * @returns any Password reset successfully
     * @throws ApiError
     */
    public resetPassword({
        requestBody,
    }: {
        requestBody: ResetPasswordRequestModel,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/auth/reset-password',
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * @returns any Email change confirmed successfully
     * @throws ApiError
     */
    public confirmEmailChange({
        requestBody,
    }: {
        requestBody: ConfirmEmailChangeRequestModel,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/auth/confirm-email-change',
            body: requestBody,
            mediaType: 'application/json',
        });
    }
}
