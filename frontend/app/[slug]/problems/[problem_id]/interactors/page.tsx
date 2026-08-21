import ProblemPageLayoutWrapper, {generateMetadata as sharedGenerateMetadata} from "../ProblemPageLayoutWrapper";

import type {Metadata} from "next";
import type {ReactNode} from "react";

type SearchParams = Promise<{
  file?: string;
  [key: string]: string | string[] | undefined;
}>;

type Props = {
  params: Promise<{ slug: string; problem_id: string }>;
  searchParams: SearchParams;
};

export const generateMetadata = async ({
  params,
}: {
  params: Promise<{ slug: string; problem_id: string }>;
}): Promise<Metadata> => {
  const {problem_id} = await params;
  return sharedGenerateMetadata(problem_id);
};

const Page = ({params, searchParams}: Props): ReactNode => {
  return (
    <ProblemPageLayoutWrapper
      activeTab="interactors"
      params={params}
      searchParams={searchParams}
    />
  );
};

export default Page;
