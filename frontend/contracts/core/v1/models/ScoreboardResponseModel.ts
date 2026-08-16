/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { ScoreboardItemModel } from './ScoreboardItemModel';
import type { ScoreboardProblemHeaderModel } from './ScoreboardProblemHeaderModel';
export type ScoreboardResponseModel = {
    contest_id: string;
    penalty_per_attempt: number;
    problems: Array<ScoreboardProblemHeaderModel>;
    items: Array<ScoreboardItemModel>;
};

