/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { MessageSubmissionCompilingCompleted } from './MessageSubmissionCompilingCompleted';
import type { MessageSubmissionCompilingStarted } from './MessageSubmissionCompilingStarted';
import type { MessageSubmissionCreated } from './MessageSubmissionCreated';
import type { MessageSubmissionQueued } from './MessageSubmissionQueued';
import type { MessageSubmissionTestCompleted } from './MessageSubmissionTestCompleted';
import type { MessageSubmissionTestingCompleted } from './MessageSubmissionTestingCompleted';
import type { MessageSubmissionTestingStarted } from './MessageSubmissionTestingStarted';
import type { MessageSubmissionTestStarted } from './MessageSubmissionTestStarted';
import type { SubmissionsMessageType } from './SubmissionsMessageType';
export type SubmissionsMessage = {
    message_type: SubmissionsMessageType;
    message?: (MessageSubmissionCreated | MessageSubmissionQueued | MessageSubmissionTestingStarted | MessageSubmissionCompilingStarted | MessageSubmissionCompilingCompleted | MessageSubmissionTestStarted | MessageSubmissionTestCompleted | MessageSubmissionTestingCompleted);
};

