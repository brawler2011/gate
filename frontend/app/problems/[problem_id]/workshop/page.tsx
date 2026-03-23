import { WorkshopEditor, WorkshopNotInitialized } from "@/components/workshop";
import { DefaultLayout } from "@/components/shared";
import { ErrorDisplay } from "@/components/shared/ErrorDisplay";
import { getProblem, listWorkshopFiles } from "@/lib/actions";
import { Metadata } from "next";
import { Suspense } from "react";

type Props = {
  params: Promise<{ problem_id: string }>;
};

export const generateMetadata = async (props: Props): Promise<Metadata> => {
  const { problem_id } = await props.params;
  const [error, response] = await getProblem(problem_id);
  if (error || !response) {
    return { title: "Редактор файлов" };
  }
  return { title: `Файлы — ${response.problem.title}` };
};

const Page = async (props: Props) => {
  const { problem_id } = await props.params;

  const [problemError] = await getProblem(problem_id);
  if (problemError) return <ErrorDisplay error={problemError} />;

  const [filesError, filesResponse] = await listWorkshopFiles(problem_id);

  return (
    <DefaultLayout
      stylesConfig={{
        main: { paddingTop: 70, paddingBottom: 0 },
      }}
    >
      {filesError?.status === 404 ? (
        <WorkshopNotInitialized problemId={problem_id} />
      ) : filesError ? (
        <ErrorDisplay error={filesError} />
      ) : (
        <Suspense>
          <WorkshopEditor problemId={problem_id} initialFiles={filesResponse?.files ?? []} />
        </Suspense>
      )}
    </DefaultLayout>
  );
};

export { Page as default };
