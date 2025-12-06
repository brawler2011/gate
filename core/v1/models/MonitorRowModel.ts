/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { MonitorProblemStatusModel } from './MonitorProblemStatusModel';
export type MonitorRowModel = {
    user_id: string;
    username: string;
    total_score: number;
    total_penalty: number;
    solved_count: number;
    problems: Array<MonitorProblemStatusModel>;
};

