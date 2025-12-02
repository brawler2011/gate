/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { SubmissionsListItemModel } from './SubmissionsListItemModel';
import type { WebSocketMessageType } from './WebSocketMessageType';
/**
 * WebSocket event sent to clients when a submission is created or updated.
 *
 * The /ws/submissions endpoint accepts the following query parameters:
 * - contestId (uuid, optional): Filter by contest ID
 * - userId (uuid, optional): Filter by user ID
 * - problemId (uuid, optional): Filter by problem ID
 * - state (integer, optional): Filter by submission state
 * - language (integer, optional): Filter by programming language
 * - sortOrder (string, required): Must be "desc" for real-time updates
 *
 * Events are only sent for the first page (page=1) when sortOrder=desc.
 * New submissions appear at the top of the list.
 *
 */
export type SubmissionWebSocketEventModel = {
    message_type: WebSocketMessageType;
    submission: SubmissionsListItemModel;
    /**
     * Optional message with additional details (e.g., test progress info)
     */
    message?: string;
};

