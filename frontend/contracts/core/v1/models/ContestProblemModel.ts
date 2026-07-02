/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { ProblemSampleModel } from './ProblemSampleModel';
export type ContestProblemModel = {
    problem_id: string;
    title: string;
    time_limit: number;
    memory_limit: number;
    position: number;
    legend_html: string;
    input_format_html: string;
    output_format_html: string;
    notes_html: string;
    scoring_html: string;
    created_at: string;
    updated_at: string;
    samples: Array<ProblemSampleModel>;
};

