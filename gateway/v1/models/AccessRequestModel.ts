/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
export type AccessRequestModel = {
    id: string;
    contest_id: string;
    user_id: string;
    username?: string;
    user_role?: string;
    status: AccessRequestModel.status;
    created_at: string;
    updated_at: string;
};
export namespace AccessRequestModel {
    export enum status {
        PENDING = 'pending',
        APPROVED = 'approved',
        REJECTED = 'rejected',
    }
}

