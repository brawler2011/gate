/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { AuthResponseModel } from '../models/AuthResponseModel';
import type { LoginRequestModel } from '../models/LoginRequestModel';
import type { RegisterRequestModel } from '../models/RegisterRequestModel';
import type { CancelablePromise } from '../core/CancelablePromise';
import type { BaseHttpRequest } from '../core/BaseHttpRequest';
export class AuthService {
    constructor(public readonly httpRequest: BaseHttpRequest) {}
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
}
