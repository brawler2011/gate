"use client";

import { createContext, useContext, useState, useTransition } from "react";

type TransitionContextType = {
  isPending: boolean;
  startTransition: (callback: () => void) => void;
  pendingView: string;
  setPendingView: (view: string) => void;
  isPaginationTransition: boolean;
  setIsPaginationTransition: (isPagination: boolean) => void;
};

// Export context so other wrappers can use it
export const TransitionContext = createContext<TransitionContextType>(null!);

export const usePageTransition = () => useContext(TransitionContext);

export const WorkshopPageWrapper = ({ children }: React.PropsWithChildren) => {
  const [isPending, startTransition] = useTransition();
  const [pendingView, setPendingView] = useState("");
  const [isPaginationTransition, setIsPaginationTransition] = useState(false);

  return (
    <TransitionContext.Provider value={{ isPending, startTransition, pendingView, setPendingView, isPaginationTransition, setIsPaginationTransition }}>
      {children}
    </TransitionContext.Provider>
  );
};
