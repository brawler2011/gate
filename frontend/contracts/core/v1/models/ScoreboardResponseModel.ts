/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { ScoreboardItemModel } from './ScoreboardItemModel';
import type { ScoreboardProblemHeaderModel } from './ScoreboardProblemHeaderModel';
export type ScoreboardResponseModel = {
    contest_id: string;
    penalty_per_attempt: number;
    /**
     * Whether the scoreboard is currently frozen
     */
    is_frozen: boolean;
    /**
     * RFC3339 timestamp when scoreboard was or will be frozen
     */
    freeze_time?: string | null;
    problems: Array<ScoreboardProblemHeaderModel>;
    items: Array<ScoreboardItemModel>;
};

