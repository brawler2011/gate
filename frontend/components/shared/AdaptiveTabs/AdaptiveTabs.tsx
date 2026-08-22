"use client";

import {Menu} from "@mantine/core";
import {IconChevronDown} from "@tabler/icons-react";
import cx from "clsx";
import Link from "next/link";
import {usePathname} from "next/navigation";
import {useState, type ReactNode} from "react";

import classes from "./AdaptiveTabs.module.css";
import {type AdaptiveTabItem, useAdaptiveTabs} from "./useAdaptiveTabs";

export type AdaptiveTabsProps = {
  items: AdaptiveTabItem[];
  className?: string;
};

export const AdaptiveTabs = ({
  items,
  className,
}: AdaptiveTabsProps): ReactNode => {
  const pathname = usePathname();
  const [opened, setOpened] = useState(false);
  const {containerRef, moreButtonRef, registerItem, visibleItems, overflowItems} =
    useAdaptiveTabs(items);

  if (items.length === 0) {
    return null;
  }

  const isItemActive = (item: AdaptiveTabItem) => {
    if (item.active !== undefined) {
      return item.active;
    }
    if (!pathname) {
      return false;
    }
    // Exact match for base URLs, prefix match for subpaths
    return pathname === item.href || (item.href !== "/" && pathname.startsWith(item.href));
  };

  return (
    <nav ref={containerRef} className={cx(classes.navContainer, className)}>
      <div className={classes.tabList}>
        {visibleItems.map((item) => {
          const Icon = item.icon;
          const active = isItemActive(item);

          return (
            <Link
              key={item.key}
              ref={(node) => registerItem(item.key, node)}
              href={item.href}
              className={cx(classes.tabLink, active && classes.tabLinkActive)}
            >
              {Icon ? <Icon size={15} /> : null}
              {item.label}
              {item.badge ? <span className={classes.tabBadge}>{item.badge}</span> : null}
            </Link>
          );
        })}
      </div>

      {overflowItems.length > 0 && (
        <Menu
          opened={opened}
          onChange={setOpened}
          position="bottom-end"
          shadow="md"
          withinPortal
        >
          <Menu.Target>
            <button
              ref={moreButtonRef}
              type="button"
              className={classes.moreButton}
              data-opened={opened || undefined}
              aria-haspopup="menu"
              aria-expanded={opened}
            >
              More
              <IconChevronDown size={14} />
            </button>
          </Menu.Target>
          <Menu.Dropdown>
            {overflowItems.map((item) => {
              const Icon = item.icon;
              const active = isItemActive(item);

              return (
                <Menu.Item
                  key={item.key}
                  component={Link}
                  href={item.href}
                  leftSection={Icon ? <Icon size={15} /> : undefined}
                  rightSection={item.badge}
                  className={cx(active && classes.menuItemActive)}
                  onClick={() => setOpened(false)}
                >
                  {item.label}
                </Menu.Item>
              );
            })}
          </Menu.Dropdown>
        </Menu>
      )}
    </nav>
  );
};
