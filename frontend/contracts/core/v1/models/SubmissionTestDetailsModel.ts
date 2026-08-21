/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { FailedTestDetailModel } from './FailedTestDetailModel';
import type { TestDetailItemModel } from './TestDetailItemModel';
export type SubmissionTestDetailsModel = {
    compiler_output?: string | null;
    error_line?: number | null;
    tests?: Array<TestDetailItemModel>;
    failed_test_details?: FailedTestDetailModel;
};

