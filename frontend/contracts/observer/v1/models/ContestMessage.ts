/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { ContestEventType } from './ContestEventType';
import type { MessageContestAnnouncementCreated } from './MessageContestAnnouncementCreated';
import type { MessageContestAnnouncementDeleted } from './MessageContestAnnouncementDeleted';
import type { MessageContestClarificationAnswered } from './MessageContestClarificationAnswered';
import type { MessageContestClarificationCreated } from './MessageContestClarificationCreated';
export type ContestMessage = {
    event_type: ContestEventType;
    payload: (MessageContestAnnouncementCreated | MessageContestAnnouncementDeleted | MessageContestClarificationCreated | MessageContestClarificationAnswered);
};

