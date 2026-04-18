"use client";

import { createContext, useContext, useState, useTransition } from "react";

export type PageTransitionContextType = {
  isPending: boolean;
  startTransition: (callback: () => void) => void;
  pendingView: string;
  setPendingView: (view: string) => void;
  isPaginationTransition: boolean;
  setIsPaginationTransition: (isPagination: boolean) => void;
};

export const PageTransitionContext =
  createContext<PageTransitionContextType | null>(null);

export const usePageTransition = () => {
  const context = useContext(PageTransitionContext);
  if (!context) {
    throw new Error(
      "usePageTransition must be used within PageTransitionProvider",
    );
  }

  return context;
};

export const useOptionalPageTransition = () =>
  useContext(PageTransitionContext);

export const PageTransitionProvider = ({
  children,
}: React.PropsWithChildren) => {
  const [isPending, startTransition] = useTransition();
  const [pendingView, setPendingView] = useState("");
  const [isPaginationTransition, setIsPaginationTransition] = useState(false);

  return (
    <PageTransitionContext.Provider
      value={{
        isPending,
        startTransition,
        pendingView,
        setPendingView,
        isPaginationTransition,
        setIsPaginationTransition,
      }}
    >
      {children}
    </PageTransitionContext.Provider>
  );
};
