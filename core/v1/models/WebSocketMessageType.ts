/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
/**
 * WebSocket message type for submission list updates:
 * - submission_created: A new submission was created
 * - submission_updated: An existing submission was updated (test progress/completion)
 *
 */
export enum WebSocketMessageType {
    SUBMISSION_CREATED = 'submission_created',
    SUBMISSION_UPDATED = 'submission_updated',
}
