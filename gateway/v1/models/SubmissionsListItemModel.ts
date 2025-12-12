/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
export type SubmissionsListItemModel = {
    id: string;
    user_id: string;
    username: string;
    state: number;
    score: number;
    penalty: number;
    time_stat: number;
    memory_stat: number;
    language: number;
    problem_id: string;
    problem_title: string;
    position: number;
    contest_id: string;
    contest_title: string;
    /**
     * The test number (1-indexed) where the submission failed. NULL for AC submissions.
     */
    failed_test?: number | null;
    updated_at: string;
    created_at: string;
};

