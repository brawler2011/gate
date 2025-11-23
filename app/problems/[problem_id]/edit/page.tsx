import {UpdateProblem} from "@/app/problems/[problem_id]/edit/actions";
import {ProblemForm} from "@/components/ProblemForm";
import {DefaultLayout} from "@/components/Layout";
import {getProblem, uploadProblemTests as uploadProblemTestsAction,} from "@/lib/actions";
import {Metadata} from "next";
import {notFound} from "next/navigation";

type Props = {
  params: Promise<{ problem_id: string }>;
};

export const generateMetadata = async (props: Props): Promise<Metadata> => {
  const { problem_id } = await props.params;
  const response = await getProblem(problem_id);

  if (!response) {
    return {
      title: "Задача не найдена",
    };
  }

  const problem = response.problem;

  return {
    title: `Редактирование ${problem.title}`,
    description: "",
  };
};

const Page = async (props: Props) => {
  const { problem_id } = await props.params;
  const response = await getProblem(problem_id);

  if (!response) {
    notFound();
  }

  const onUploadFn = async (id: string, data: FormData) => {
    "use server";

    const file = data.get("file");
    if (!file || !(file instanceof File)) {
      throw new Error("Invalid file");
    }

    try {
        return await uploadProblemTestsAction(id, file);
    } catch (error) {
      console.error("Failed to upload problem tests:", error);
      throw error;
    }
  };

  const problem = response.problem;

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
