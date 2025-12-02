/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
/**
 * WebSocket message type for submission list updates:
 * - submission_created: A new submission was created
 * - submission_updated: An existing submission was updated (test results)
 * - testing_started: Testing of a submission has started
 * - test_completed: A single test case completed (shows progress)
 * - testing_completed: All testing for a submission is finished
 *
 */
export enum WebSocketMessageType {
    SUBMISSION_CREATED = 'submission_created',
    SUBMISSION_UPDATED = 'submission_updated',
    TESTING_STARTED = 'testing_started',
    TEST_COMPLETED = 'test_completed',
    TESTING_COMPLETED = 'testing_completed',
}
