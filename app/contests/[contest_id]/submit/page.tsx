import { Footer } from "@/components/Footer";
import { HeaderWithSession } from "@/components/HeaderWithSession";
import { Layout } from "@/components/Layout";
import { ErrorDisplay } from "@/components/ErrorDisplay";
import { getContest } from "@/lib/actions";
import {
  AppShellFooter,
  AppShellHeader,
  AppShellMain,
  Container,
} from "@mantine/core";
import { Metadata } from "next";
import { ContestHotbar } from "@/components/ContestHotbar";
import { SubmitSubmissionClient } from "./SubmitSubmissionClient";
import { getCurrentUser } from "@/lib/auth";
import { getMyContestRole } from "@/lib/contest-role";

type Props = {
  params: Promise<{ contest_id: string }>;
};

export const generateMetadata = async ({
  params,
}: Props): Promise<Metadata> => {
  const { contest_id } = await params;

  const [error, response] = await getContest(contest_id);
  if (error || !response) {
    return {
      title: "Ошибка загрузки контеста",
    };
  }

  return {
    title: response.contest.title,
    description: response.contest.title,
  };
};

const Page = async ({ params }: Props) => {
  const { contest_id } = await params;

  const [error, response] = await getContest(contest_id);
  if (error) return <ErrorDisplay error={error} />;

  // Get user and contest role for permissions
  const user = await getCurrentUser();
  const contestRole = user ? await getMyContestRole(contest_id) : null;

  return (
    <Layout>
      <AppShellHeader>
        <HeaderWithSession />
      </AppShellHeader>
      <AppShellMain>
        <Container
          size="lg"
          pt={0}
          pb={{ base: "md", sm: "lg", md: "xl" }}
          px={{ base: "xs", sm: "md", md: "lg" }}
        >
          <ContestHotbar 
            contest={response!.contest}
            user={user}
            contestRole={contestRole}
            activeTab="submit"
          />
          <SubmitSubmissionClient 
            contest={response!.contest}
            problems={response!.problems || []}
            user={user}
          />
        </Container>
      </AppShellMain>
      <AppShellFooter withBorder={false}>
        <Footer />
      </AppShellFooter>
    </Layout>
  );
};

export default Page;
