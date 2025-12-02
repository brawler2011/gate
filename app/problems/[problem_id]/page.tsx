"use server";

import { DefaultLayout } from "@/components/Layout";
import { Problem } from "@/components/Problem";
import { getProblem } from "@/lib/actions";
import { Metadata } from "next";
import { Stack } from "@mantine/core";

type Props = {
  params: Promise<{ problem_id: string }>;
};

export const generateMetadata = async (props: Props): Promise<Metadata> => {
  const { problem_id } = await props.params;
  
  try {
    const response = await getProblem(problem_id);
    return {
      title: `${response.problem.title}`,
      description: "",
    };
  } catch {
    return {
      title: "Ошибка загрузки задачи",
    };
  }
};

const Page = async (props: Props) => {
  const { problem_id } = await props.params;
  const response = await getProblem(problem_id);

  return (
    <DefaultLayout>
      <Stack px="16" pb="16" maw="1920px" m="0 auto">
        <Stack align="center" w="fit-content" gap="16" m="0 auto">
          <Problem problem={response.problem} letter="A" />
        </Stack>
      </Stack>
    </DefaultLayout>
  );
};

export default Page;
