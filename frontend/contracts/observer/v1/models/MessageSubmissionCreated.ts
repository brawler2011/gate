/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { SubmissionEventMeta } from './SubmissionEventMeta';
export type MessageSubmissionCreated = (SubmissionEventMeta & {
    frozen?: boolean;
    id: string;
    state?: number;
    source?: string;
});

