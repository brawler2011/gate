"use client";

import {
  PageTransitionProvider,
  usePageTransition,
} from "@/components/shared/PageTransitionContext";

export { usePageTransition };

export const ContestsPageWrapper = ({ children }: React.PropsWithChildren) => {
  return <PageTransitionProvider>{children}</PageTransitionProvider>;
};
