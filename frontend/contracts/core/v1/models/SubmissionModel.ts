/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { SubmissionTestDetailsModel } from './SubmissionTestDetailsModel';
export type SubmissionModel = {
    id: string;
    user_id: string;
    username: string;
    submission: string;
    state: number;
    score: number;
    penalty: number;
    time_stat: number;
    memory_stat: number;
    language: number;
    failed_test?: number | null;
    test_details?: SubmissionTestDetailsModel;
    problem_id: string;
    problem_title: string;
    position: number;
    contest_id: string;
    contest_login: string;
    contest_title: string;
    organization_login?: string;
    updated_at: string;
    created_at: string;
};

