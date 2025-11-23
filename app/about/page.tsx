import { DefaultLayout } from "@/components/Layout";
import { PresentationSlide } from "@/components/PresentationSlide";

export const metadata = {
  title: "О проекте",
};

export default function Page() {
  return (
    <DefaultLayout>
      <PresentationSlide />
    </DefaultLayout>
  );
}
