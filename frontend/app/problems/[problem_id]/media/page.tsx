import ProblemPage, { generateMetadata as sharedGenerateMetadata } from "../ProblemPage";

type SearchParams = Promise<{
  file?: string;
  [key: string]: string | string[] | undefined;
}>;

type Props = {
  params: Promise<{ problem_id: string }>;
  searchParams: SearchParams;
};

export const generateMetadata = sharedGenerateMetadata;

export default async function Page({ params, searchParams }: Props) {
  return <ProblemPage params={params} searchParams={searchParams} activeTab="media" />;
}
