/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { FileStatus } from './FileStatus';
export type WorkshopStatus = {
    current_sha?: string;
    modified_files?: Array<FileStatus>;
    has_uncommitted_changes?: boolean;
};

