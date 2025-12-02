"use server";

import { DefaultLayout } from "@/components/Layout";
import { Problem } from "@/components/Problem";
import { ErrorDisplay } from "@/components/ErrorDisplay";
import { getProblem } from "@/lib/actions";
import { Metadata } from "next";
import { Stack } from "@mantine/core";

type Props = {
  params: Promise<{ problem_id: string }>;
};

export const generateMetadata = async (props: Props): Promise<Metadata> => {
  const { problem_id } = await props.params;
  
  const [error, response] = await getProblem(problem_id);
  if (error || !response) {
    return {
      title: "Ошибка загрузки задачи",
    };
  }

  return {
    title: `${response.problem.title}`,
    description: "",
  };
};

const Page = async (props: Props) => {
  const { problem_id } = await props.params;
  const [error, response] = await getProblem(problem_id);

  if (error) return <ErrorDisplay error={error} />;

  return (
    <DefaultLayout>
      <Stack px="16" pb="16" maw="1920px" m="0 auto">
        <Stack align="center" w="fit-content" gap="16" m="0 auto">
          <Problem problem={response!.problem} letter="A" />
        </Stack>
      </Stack>
    </DefaultLayout>
  );
};

export default Page;
