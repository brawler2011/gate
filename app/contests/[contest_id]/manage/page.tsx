import {ParticipantsSection} from "@/components/ContestManage/ParticipantsSection";
import {ProblemsSection} from "@/components/ContestManage/ProblemsSection";
import {SettingsSection} from "@/components/ContestManage/SettingsSection";
import {DefaultLayout} from "@/components/Layout";
import {getContest} from "@/lib/actions";
import {CONTEST_CONTENT_MAX_WIDTH} from "@/lib/constants";
import {Box, Button, Container, Group, Stack, Title} from "@mantine/core";
import {IconArrowLeft, IconPuzzle, IconSettings, IconUsers} from "@tabler/icons-react";
import Link from "next/link";
import type {ContestProblemListItemModel} from "../../../../../contracts/core/v1";

// Constants for sections
const SECTIONS = {
    SETTINGS: "settings",
    PROBLEMS: "problems",
    PARTICIPANTS: "participants",
} as const;

type Section = typeof SECTIONS[keyof typeof SECTIONS];

// Navigation configuration
const NAV_SECTIONS = [
    {
        key: SECTIONS.SETTINGS,
        label: "Настройки",
        icon: IconSettings,
    },
    {
        key: SECTIONS.PROBLEMS,
        label: "Задачи",
        icon: IconPuzzle,
    },
    {
        key: SECTIONS.PARTICIPANTS,
        label: "Участники",
        icon: IconUsers,
    },
] as const;

type Props = {
    params: Promise<{ contest_id: string }>;
    searchParams: Promise<{ section?: string }>;
};

export default async function ContestManagePage({params, searchParams}: Props) {
    const {contest_id: contestId} = await params;
    const {section = "settings"} = await searchParams;

    const response = await getContest(contestId);
    const contest = response.contest;
    const problems: Array<ContestProblemListItemModel> = response.problems || [];

    const validSections = Object.values(SECTIONS);
    const activeSection = (
        validSections.includes(section as Section)
            ? section
            : SECTIONS.SETTINGS
    ) as Section;

    return (
        <DefaultLayout>
            <Container
                size="lg"
                pt={0}
                pb={{base: "md", sm: "lg", md: "xl"}}
                px={{base: "xs", sm: "md", md: "lg"}}
            >
                {/* Header Section */}
                <Stack gap="md" mb="lg" style={{maxWidth: CONTEST_CONTENT_MAX_WIDTH, margin: "0 auto"}}>
                    {/* Заголовок с кнопкой "Назад" */}
                    <Group justify="space-between" align="center" wrap="nowrap">
                        <Title order={1} size="h3">
                            🏆 {contest.title}
                        </Title>
                        <Button
                            component={Link}
                            href={`/contests/${contestId}`}
                            variant="default"
                            size="sm"
                            leftSection={<IconArrowLeft size={16}/>}
                            visibleFrom="sm"
                            style={{flexShrink: 0}}
                        >
                            Назад к контесту
                        </Button>
                    </Group>
                    
                    {/* Кнопки управления разделами */}
                    <Group gap="sm">
                        {NAV_SECTIONS.map((section) => {
                            const Icon = section.icon;
                            return (
                                <Button
                                    key={section.key}
                                    component={Link}
                                    href={`/contests/${contestId}/manage?section=${section.key}`}
                                    variant={activeSection === section.key ? "filled" : "default"}
                                    size="sm"
                                    leftSection={<Icon size={16}/>}
                                    visibleFrom="sm"
                                >
                                    {section.label}
                                </Button>
                            );
                        })}
                    </Group>
                </Stack>

                {/* Content Area */}
                <Box style={{maxWidth: CONTEST_CONTENT_MAX_WIDTH, margin: "0 auto"}}>
                    {activeSection === SECTIONS.SETTINGS && (
                        <SettingsSection contest={contest}/>
                    )}
                    {activeSection === SECTIONS.PROBLEMS && (
                        <ProblemsSection
                            contestId={contestId}
                            initialProblems={problems}
                        />
                    )}
                    {activeSection === SECTIONS.PARTICIPANTS && (
                        <ParticipantsSection contestId={contestId}/>
                    )}
                </Box>
            </Container>
        </DefaultLayout>
    );
}
