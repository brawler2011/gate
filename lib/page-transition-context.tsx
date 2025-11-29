"use client";

import { createContext, useContext, useState, useTransition } from "react";

export type TransitionContextType = {
  isPending: boolean;
  startTransition: (callback: () => void) => void;
  pendingView: string;
  setPendingView: (view: string) => void;
  isPaginationTransition: boolean;
  setIsPaginationTransition: (isPagination: boolean) => void;
};

export const TransitionContext = createContext<TransitionContextType | null>(null);

export const usePageTransition = () => useContext(TransitionContext);

export const PageTransitionProvider = ({ children }: React.PropsWithChildren) => {
  const [isPending, startTransition] = useTransition();
  const [pendingView, setPendingView] = useState("");
  const [isPaginationTransition, setIsPaginationTransition] = useState(false);

  return (
    <TransitionContext.Provider value={{ isPending, startTransition, pendingView, setPendingView, isPaginationTransition, setIsPaginationTransition }}>
      {children}
    </TransitionContext.Provider>
  );
};

