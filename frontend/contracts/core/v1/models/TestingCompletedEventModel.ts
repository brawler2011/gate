/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { TestProgressEventType } from './TestProgressEventType';
export type TestingCompletedEventModel = {
    type: TestProgressEventType;
    /**
     * UUID of the submission
     */
    submission_id: string;
    /**
     * Total number of test cases
     */
    total_tests: number;
    /**
     * Final submission state:
     * - 1: Saved (not yet tested)
     * - 101: Compilation Error (CE)
     * - 102: Time Limit Exceeded (TL)
     * - 103: Memory Limit Exceeded (ML)
     * - 104: Runtime Error (RE)
     * - 105: Presentation Error (PE)
     * - 106: Wrong Answer (WA)
     * - 200: Accepted (AC)
     *
     */
    state: TestingCompletedEventModel.state;
};
export namespace TestingCompletedEventModel {
    /**
     * Final submission state:
     * - 1: Saved (not yet tested)
     * - 101: Compilation Error (CE)
     * - 102: Time Limit Exceeded (TL)
     * - 103: Memory Limit Exceeded (ML)
     * - 104: Runtime Error (RE)
     * - 105: Presentation Error (PE)
     * - 106: Wrong Answer (WA)
     * - 200: Accepted (AC)
     *
     */
    export enum state {
        '_1' = 1,
        '_101' = 101,
        '_102' = 102,
        '_103' = 103,
        '_104' = 104,
        '_105' = 105,
        '_106' = 106,
        '_200' = 200,
    }
}

