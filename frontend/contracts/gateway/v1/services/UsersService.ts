/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { PatchMeRequestModel } from '../models/PatchMeRequestModel';
import type { CancelablePromise } from '../core/CancelablePromise';
import type { BaseHttpRequest } from '../core/BaseHttpRequest';
export class UsersService {
    constructor(public readonly httpRequest: BaseHttpRequest) {}
    /**
     * Update current user profile
     * @returns any Profile updated successfully
     * @throws ApiError
     */
    public patchMe({
        requestBody,
    }: {
        requestBody: PatchMeRequestModel,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/users/me',
            body: requestBody,
            mediaType: 'application/json',
        });
    }
}
