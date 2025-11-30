/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { TestProgressEventType } from './TestProgressEventType';
export type TestingStartedEventModel = {
    type: TestProgressEventType;
    /**
     * UUID of the submission being tested
     */
    submission_id: string;
    /**
     * Total number of test cases for this submission
     */
    total_tests: number;
};

