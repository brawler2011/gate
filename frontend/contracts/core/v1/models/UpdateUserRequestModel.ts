/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
export type UpdateUserRequestModel = {
    username?: string;
    role?: UpdateUserRequestModel.role;
    email?: string;
};
export namespace UpdateUserRequestModel {
    export enum role {
        USER = 'user',
        ADMIN = 'admin',
    }
}

