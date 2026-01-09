/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { CancelablePromise } from '../core/CancelablePromise';
import type { BaseHttpRequest } from '../core/BaseHttpRequest';
export class DefaultService {
    constructor(public readonly httpRequest: BaseHttpRequest) {}
    /**
     * @throws ApiError
     */
    public observeSubmissions({
        since,
        contestId,
        userId,
        problemId,
        language,
    }: {
        since: number,
        contestId?: string,
        userId?: string,
        problemId?: string,
        language?: number,
    }): CancelablePromise<void> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/ws/submissions',
            query: {
                'since': since,
                'contestId': contestId,
                'userId': userId,
                'problemId': problemId,
                'language': language,
            },
        });
    }
}
