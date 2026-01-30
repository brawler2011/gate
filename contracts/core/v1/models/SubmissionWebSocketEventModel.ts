/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { SubmissionsListItemModel } from './SubmissionsListItemModel';
import type { WebSocketMessageType } from './WebSocketMessageType';
/**
 * WebSocket event sent to clients for submission list updates.
 *
 * Event types:
 * - submission_created: New submission (includes full submission data)
 * - submission_updated: Test results ready (includes full submission data)
 * - testing_started: Testing began (includes submission_id, total_tests)
 * - test_completed: Test finished (includes submission_id, test_number, total_tests, passed)
 * - testing_completed: All tests done (includes submission_id, total_tests, state)
 *
 * The /ws/submissions endpoint accepts these query parameters:
 * - contestId (uuid, optional): Filter by contest ID
 * - userId (uuid, optional): Filter by user ID
 * - problemId (uuid, optional): Filter by problem ID
 * - state (integer, optional): Filter by submission state
 * - language (integer, optional): Filter by programming language
 * - sortOrder (string, required): Must be "desc" for real-time updates
 *
 */
export type SubmissionWebSocketEventModel = {
    message_type: WebSocketMessageType;
    /**
     * Full submission data (for submission_created/submission_updated events)
     */
    submission?: SubmissionsListItemModel;
    /**
     * Optional message with additional details
     */
    message?: string;
    /**
     * Submission ID (for test progress events)
     */
    submission_id?: string;
    /**
     * Current test number (for test_completed events)
     */
    test_number?: number;
    /**
     * Total number of tests
     */
    total_tests?: number;
    /**
     * Whether the current test passed (for test_completed events)
     */
    passed?: boolean;
    /**
     * Final submission state (for testing_completed events)
     */
    state?: number;
    /**
     * Contest ID (for filtering)
     */
    contest_id?: string;
    /**
     * User ID (for filtering)
     */
    user_id?: string;
    /**
     * Problem ID (for filtering)
     */
    problem_id?: string;
    /**
     * Programming language (for filtering)
     */
    language?: number;
};

