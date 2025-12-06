/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
export type InvitationModel = {
    id: string;
    contest_id: string;
    user_id: string;
    username?: string;
    user_role?: string;
    invited_by: string;
    invited_by_username?: string;
    status: InvitationModel.status;
    created_at: string;
    updated_at: string;
};
export namespace InvitationModel {
    export enum status {
        PENDING = 'pending',
        ACCEPTED = 'accepted',
        DECLINED = 'declined',
        REVOKED = 'revoked',
    }
}

