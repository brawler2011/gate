/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
export type GetMyContestRoleResponseModel = {
    role: string;
    /**
     * Effective permissions bitmask for the user in this contest, derived from direct membership and team-based access
     */
    permissions_mask?: number;
};

