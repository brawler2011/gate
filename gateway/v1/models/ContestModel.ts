/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { UserModel } from './UserModel';
export type ContestModel = {
    id: string;
    title: string;
    description: string;
    visibility: string;
    monitor_scope: string;
    submissions_list_scope: string;
    submissions_review_scope: string;
    created_by: string;
    owner?: UserModel;
    start_time?: string | null;
    end_time?: string | null;
    scoring_mode: ContestModel.scoring_mode;
    created_at: string;
    updated_at: string;
};
export namespace ContestModel {
    export enum scoring_mode {
        POINTS = 'points',
        BINARY = 'binary',
    }
}

