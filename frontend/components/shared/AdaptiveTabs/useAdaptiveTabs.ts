"use client";

import {useCallback, useEffect, useRef, useState} from "react";

export type AdaptiveTabItem = {
  key: string;
  label: string;
  href: string;
  icon?: React.ComponentType<{ size?: string | number }>;
  active?: boolean;
};

export type UseAdaptiveTabsResult = {
  containerRef: React.RefObject<HTMLElement>;
  moreButtonRef: React.RefObject<HTMLButtonElement>;
  registerItem: (key: string, node: HTMLElement | null) => void;
  visibleItems: AdaptiveTabItem[];
  overflowItems: AdaptiveTabItem[];
};

export const useAdaptiveTabs = (
  items: AdaptiveTabItem[],
  gap = 0,
): UseAdaptiveTabsResult => {
  const containerRef = useRef<HTMLElement>(null);
  const moreButtonRef = useRef<HTMLButtonElement>(null);
  const itemWidthsRef = useRef<Map<string, number>>(new Map());
  const [visibleCount, setVisibleCount] = useState<number>(items.length);

  const registerItem = useCallback((key: string, node: HTMLElement | null) => {
    if (node) {
      const width = node.offsetWidth;
      if (width > 0) {
        itemWidthsRef.current.set(key, width);
      }
    }
  }, []);

  const recalculate = useCallback(() => {
    const container = containerRef.current;
    if (!container || items.length === 0) {
      setVisibleCount(items.length);
      return;
    }

    const containerWidth = container.clientWidth;
    if (containerWidth === 0) {
      setVisibleCount(items.length);
      return;
    }

    // Calculate total width if all items were visible
    let totalWidth = 0;
    const widths: number[] = [];
    for (let i = 0; i < items.length; i++) {
      const w = itemWidthsRef.current.get(items[i].key) ?? 0;
      widths.push(w);
      totalWidth += w + (i > 0 ? gap : 0);
    }

    // If some items haven't been measured yet, show all initially
    if (widths.some((w) => w === 0)) {
      setVisibleCount(items.length);
      return;
    }

    // If everything fits, no More button needed
    if (totalWidth <= containerWidth) {
      setVisibleCount(items.length);
      return;
    }

    // Space needed for "More" button
    const moreBtnWidth = moreButtonRef.current?.offsetWidth || 70;
    const availableWidth = containerWidth - moreBtnWidth - gap;

    let usedWidth = 0;
    let count = 0;

    for (let i = 0; i < items.length; i++) {
      const itemWidth = widths[i];
      const nextWidth = usedWidth + itemWidth + (count > 0 ? gap : 0);
      if (nextWidth > availableWidth) {
        break;
      }
      usedWidth = nextWidth;
      count++;
    }

    setVisibleCount(Math.max(1, count));
  }, [items, gap]);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) {
      return;
    }

    const observer = new ResizeObserver(() => {
      recalculate();
    });
    observer.observe(container);

    // Initial calculation
    recalculate();

    return () => {
      observer.disconnect();
    };
  }, [recalculate]);

  return {
    containerRef,
    moreButtonRef,
    registerItem,
    visibleItems: items.slice(0, visibleCount),
    overflowItems: items.slice(visibleCount),
  };
};
