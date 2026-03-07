/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { PaginationModel } from './PaginationModel';
import type { SubmissionsListItemModel } from './SubmissionsListItemModel';
export type ListSubmissionsResponseModel = {
    'access-token'?: string;
    submissions: Array<SubmissionsListItemModel>;
    pagination: PaginationModel;
};

