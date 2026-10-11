/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { SubmissionEventMeta } from './SubmissionEventMeta';
export type MessageSubmissionCompleted = (SubmissionEventMeta & {
    frozen?: boolean;
    id: string;
    state?: number;
    score?: number;
    penalty?: number;
    time_stat?: number;
    memory_stat?: number;
    failed_test?: number | null;
});

