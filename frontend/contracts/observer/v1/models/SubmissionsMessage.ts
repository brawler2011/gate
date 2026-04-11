/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { MessageSubmissionCompilingStarted } from './MessageSubmissionCompilingStarted';
import type { MessageSubmissionCompleted } from './MessageSubmissionCompleted';
import type { MessageSubmissionCreated } from './MessageSubmissionCreated';
import type { MessageSubmissionQueued } from './MessageSubmissionQueued';
import type { MessageSubmissionTestingStarted } from './MessageSubmissionTestingStarted';
import type { MessageSubmissionTestStarted } from './MessageSubmissionTestStarted';
import type { SubmissionsEventType } from './SubmissionsEventType';
export type SubmissionsMessage = {
    event_type: SubmissionsEventType;
    payload: (MessageSubmissionCreated | MessageSubmissionQueued | MessageSubmissionCompilingStarted | MessageSubmissionTestingStarted | MessageSubmissionTestStarted | MessageSubmissionCompleted);
};

