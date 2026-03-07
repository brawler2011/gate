/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { TestProgressEventType } from './TestProgressEventType';
export type TestCompletedEventModel = {
    type: TestProgressEventType;
    /**
     * UUID of the submission being tested
     */
    submission_id: string;
    /**
     * The test case number that just completed (1-indexed)
     */
    test_number: number;
    /**
     * Total number of test cases
     */
    total_tests: number;
    /**
     * Whether the test case passed
     */
    passed: boolean;
};

