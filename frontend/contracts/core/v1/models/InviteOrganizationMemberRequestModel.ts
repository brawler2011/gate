/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
export type InviteOrganizationMemberRequestModel = {
    user_id: string;
    role: InviteOrganizationMemberRequestModel.role;
};
export namespace InviteOrganizationMemberRequestModel {
    export enum role {
        OWNER = 'owner',
        ADMIN = 'admin',
        MEMBER = 'member',
    }
}

