/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { PaginationModel } from './PaginationModel';
import type { SubmissionsListItemModel } from './SubmissionsListItemModel';
export type ListSubmissionsResponseModel = {
    since?: number;
    submissions: Array<SubmissionsListItemModel>;
    pagination: PaginationModel;
};

