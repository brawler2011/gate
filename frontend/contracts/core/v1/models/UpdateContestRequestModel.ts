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
    /**
     * Freeze duration in minutes before contest end
     */
    freeze_duration_minutes?: number | null;
    /**
     * Freeze mode status
     */
    freeze_status?: UpdateContestRequestModel.freeze_status;
    start_time?: string | null;
    end_time?: string | null;
};
export namespace UpdateContestRequestModel {
    /**
     * Freeze mode status
     */
    export enum freeze_status {
        AUTO = 'auto',
        FROZEN = 'frozen',
        UNFROZEN = 'unfrozen',
    }
}

