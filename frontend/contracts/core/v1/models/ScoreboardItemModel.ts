/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { ScoreboardProblemResultModel } from './ScoreboardProblemResultModel';
export type ScoreboardItemModel = {
    user_id: string;
    username: string;
    problems_solved: number;
    total_penalty: number;
    last_accepted_at?: string;
    problem_results: Array<ScoreboardProblemResultModel>;
};

