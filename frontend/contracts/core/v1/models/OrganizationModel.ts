/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
export type OrganizationModel = {
    id: string;
    login: string;
    name: string;
    description?: string;
    join_policy: OrganizationModel.join_policy;
    created_at: string;
    updated_at: string;
};
export namespace OrganizationModel {
    export enum join_policy {
        OPEN = 'open',
        BY_REQUEST = 'by_request',
        INVITE_ONLY = 'invite_only',
    }
}

