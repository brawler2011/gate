import ProblemPageLayoutWrapper, { generateMetadata as sharedGenerateMetadata } from "./ProblemPageLayoutWrapper";
import type { Metadata } from "next";

type SearchParams = Promise<{
  file?: string;
  [key: string]: string | string[] | undefined;
}>;

type Props = {
  params: Promise<{ problem_id: string }>;
  searchParams: SearchParams;
};

export const generateMetadata = async (props: { params: Promise<{ problem_id: string }> }): Promise<Metadata> => {
  const { problem_id } = await props.params;
  return sharedGenerateMetadata(problem_id);
};

export default function Page({ params, searchParams }: Props) {
  return (
    <ProblemPageLayoutWrapper
      activeTab="general"
      params={params}
      searchParams={searchParams}
    />
  );
}
