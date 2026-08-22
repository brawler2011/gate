/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
export type UpdateOrganizationRequestModel = {
    login?: string;
    name?: string;
    description?: string;
    join_policy?: UpdateOrganizationRequestModel.join_policy;
};
export namespace UpdateOrganizationRequestModel {
    export enum join_policy {
        OPEN = 'open',
        BY_REQUEST = 'by_request',
        INVITE_ONLY = 'invite_only',
    }
}

