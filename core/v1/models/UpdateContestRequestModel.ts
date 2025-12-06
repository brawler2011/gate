/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
export type UpdateContestRequestModel = {
    title?: string;
    description?: string;
    visibility?: string;
    monitor_scope?: string;
    submissions_list_scope?: string;
    submissions_review_scope?: string;
    start_time?: string;
    end_time?: string;
    scoring_mode?: UpdateContestRequestModel.scoring_mode;
};
export namespace UpdateContestRequestModel {
    export enum scoring_mode {
        POINTS = 'points',
        BINARY = 'binary',
    }
}

