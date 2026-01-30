/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
/**
 * WebSocket testing progress event type:
 * - testing_started: Sent when testing of a submission begins
 * - test_completed: Sent after each individual test case completes
 * - testing_completed: Sent when all testing for a submission is finished
 *
 */
export enum TestProgressEventType {
    TESTING_STARTED = 'testing_started',
    TEST_COMPLETED = 'test_completed',
    TESTING_COMPLETED = 'testing_completed',
}
