import {UpdateProblem} from "@/app/problems/[problem_id]/edit/actions";
import {ProblemForm} from "@/components/ProblemForm";
import {DefaultLayout} from "@/components/Layout";
import {ErrorDisplay} from "@/components/ErrorDisplay";
import {getProblem, uploadProblemTests as uploadProblemTestsAction,} from "@/lib/actions";
import {Metadata} from "next";

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
    title: `Редактирование ${response.problem.title}`,
    description: "",
  };
};

const Page = async (props: Props) => {
  const { problem_id } = await props.params;
  const [error, response] = await getProblem(problem_id);

  if (error) return <ErrorDisplay error={error} />;

  const onUploadFn = async (id: string, data: FormData) => {
    "use server";

    const file = data.get("file");
    if (!file || !(file instanceof File)) {
      return [{ status: 400, message: "Invalid file" }, null] as const;
    }

    return await uploadProblemTestsAction(id, file);
  };

  const problem = response!.problem;

  return (
    <DefaultLayout
      stylesConfig={{
        footer: {
          position: "static",
          bottom: "auto",
          width: "100%",
          zIndex: "auto",
        },
        main: {
          paddingTop: 70,
          paddingBottom: `var(--mantine-spacing-lg)`,
        },
      }}
    >
      <ProblemForm
        problem={problem}
        onSubmitFn={UpdateProblem}
        onUploadFn={onUploadFn}
      />
    </DefaultLayout>
  );
};

export { Page as default };
