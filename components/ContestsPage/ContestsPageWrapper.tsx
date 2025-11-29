"use client";

import { useState, useTransition } from "react";
import { TransitionContext, usePageTransition } from "../WorkshopPage/WorkshopPageWrapper";

export { usePageTransition };

export const ContestsPageWrapper = ({ children }: React.PropsWithChildren) => {
  const [isPending, startTransition] = useTransition();
  const [pendingView, setPendingView] = useState("");
  const [isPaginationTransition, setIsPaginationTransition] = useState(false);

  return (
    <TransitionContext.Provider value={{ isPending, startTransition, pendingView, setPendingView, isPaginationTransition, setIsPaginationTransition }}>
      {children}
    </TransitionContext.Provider>
  );
};
