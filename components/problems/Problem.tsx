'use client';

import {Stack, Text, Title} from "@mantine/core";
import {useEffect, useRef} from 'react';
import katex from 'katex';
import 'katex/dist/katex.min.css';
import './Problem.css';

type Props = {
    problem: {
        // id: number,
        title: string,
        time_limit: number,
        memory_limit: number,
        legend_html: string,
        input_format_html: string,
        output_format_html: string,
        notes_html: string,
        scoring_html: string,
        created_at: string,
        updated_at: string
    }

    letter?: string
}

const prettifyTimeLimit = (time_limit: number) => {
    if (time_limit % 1000 === 0) {
        return `${time_limit / 1000} сек`
    }

    return `${time_limit} мс`
}

const prettifyMemoryLimit = (memory_limit: number) => {
    if (memory_limit % 1000 === 0) {
        return `${memory_limit / 1000} ГБ`
    }

    return `${memory_limit} МБ`
}

const Problem = ({problem, letter}: Props) => {
    letter = letter || 'A';

    const ref = useRef<HTMLDivElement>(null);

    useEffect(() => {
        if (ref.current) {
            const mathElements = ref.current.querySelectorAll('.math');
            mathElements.forEach((element) => {
                if (element instanceof HTMLElement && !element.hasAttribute('data-rendered')) {
                    katex.render(element.textContent || '', element, {
                        throwOnError: false,
                        displayMode: element.classList.contains('display'),
                    });

                    // mark as rendered
                    element.setAttribute('data-rendered', 'true');
                }
            });
        }

        return () => {
            if (ref.current) {
                ref.current.querySelectorAll('.math').forEach((element) => {
                    element.removeAttribute('data-rendered');
                });
            }
        };
    }, [problem, letter]);

    return (
        <Stack className="container" ref={ref} gap="md">
            <Stack align="center" gap={0} w="fit-content" mx="auto" mb="sm">
                <Title order={2}>{letter}. {problem.title}</Title>
                <Stack align="center" gap={0}>
                    <Text>
                        ограничение по времени: {prettifyTimeLimit(problem.time_limit)}
                    </Text>
                    <Text>
                        ограничение по памяти: {prettifyMemoryLimit(problem.memory_limit)}
                    </Text>
                </Stack>
            </Stack>
            {problem.legend_html && (
                <div className="content" dangerouslySetInnerHTML={{__html: problem.legend_html}}/>
            )}
            {problem.input_format_html && (
                <Stack gap="xs">
                    <Title order={3}>Входные данные</Title>
                    <div className="content" dangerouslySetInnerHTML={{__html: problem.input_format_html}}/>
                </Stack>
            )}
            {problem.output_format_html && (
                <Stack gap="xs">
                    <Title order={3}>Выходные данные</Title>
                    <div className="content" dangerouslySetInnerHTML={{__html: problem.output_format_html}}/>
                </Stack>
            )}
            {problem.scoring_html && (
                <Stack gap="xs">
                    <Title order={3}>Система оценки</Title>
                    <div className="content" dangerouslySetInnerHTML={{__html: problem.scoring_html}}/>
                </Stack>
            )}
            {problem.notes_html && (
                <Stack gap="xs">
                    <Title order={3}>Примечание</Title>
                    <div className="content" dangerouslySetInnerHTML={{__html: problem.notes_html}}/>
                </Stack>
            )}
        </Stack>
    );
}

export {Problem};
