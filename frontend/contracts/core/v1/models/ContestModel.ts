/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { UserModel } from './UserModel';
export type ContestModel = {
    id: string;
    organization_id?: string;
    title: string;
    description: string;
    visibility: string;
    monitor_scope: string;
    submissions_list_scope: string;
    submissions_review_scope: string;
    created_by: string;
    owner?: UserModel;
    created_at: string;
    updated_at: string;
    start_time?: string | null;
    end_time?: string | null;
};

