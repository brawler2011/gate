import {ParticipantsSection} from "@/components/ContestManage/ParticipantsSection";
import {ProblemsSection} from "@/components/ContestManage/ProblemsSection";
import {SettingsSection} from "@/components/ContestManage/SettingsSection";
import {DefaultLayout} from "@/components/Layout";
import {ErrorDisplay} from "@/components/ErrorDisplay";
import {getContest} from "@/lib/actions";
import {CONTEST_CONTENT_MAX_WIDTH} from "@/lib/constants";
import {Box, Container, Stack, Title} from "@mantine/core";
import {IconArrowLeft, IconPuzzle, IconSettings, IconUsers} from "@tabler/icons-react";
import Link from "next/link";
import type {ContestProblemListItemModel} from "../../../../../contracts/core/v1";
import classes from "./styles.module.css";

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

    const [error, response] = await getContest(contestId);
    if (error) return <ErrorDisplay error={error} />;

    const contest = response!.contest;
    const problems: Array<ContestProblemListItemModel> = response!.problems || [];

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
                <Stack gap="md" style={{maxWidth: CONTEST_CONTENT_MAX_WIDTH, margin: "0 auto"}}>
                    {/* Tab Row */}
                    <div className={classes.tabRow}>
                        {/* Back to Contest Tab */}
                        <Link
                            href={`/contests/${contestId}`}
                            className={classes.tab}
                        >
                            <IconArrowLeft size={16} />
                            К контесту
                        </Link>

                        {/* Section Tabs */}
                        {NAV_SECTIONS.map((section) => {
                            const Icon = section.icon;
                            const isActive = activeSection === section.key;
                            return (
                                <Link
                                    key={section.key}
                                    href={`/contests/${contestId}/manage?section=${section.key}`}
                                    className={`${classes.tab} ${isActive ? classes.tabActive : ""}`}
                                >
                                    <Icon size={16} />
                                    {section.label}
                                </Link>
                            );
                        })}
                    </div>
                </Stack>

                {/* Content Area */}
                <Box 
                    className={classes.contentPanel}
                    style={{maxWidth: CONTEST_CONTENT_MAX_WIDTH, margin: "0 auto"}}
                >
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
