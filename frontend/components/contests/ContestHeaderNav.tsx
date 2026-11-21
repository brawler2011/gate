import {IconDeviceDesktop, IconFileCode, IconMail, IconMessageDots, IconPuzzle, IconSend, IconSettings, IconUser} from "@tabler/icons-react";
import {type ReactNode} from "react";
import {useLocation} from "react-router-dom";

import {AdaptiveTabs, type AdaptiveTabItem} from "@/components/shared/AdaptiveTabs";
import {useSession} from "@/contexts/SessionContext";
import {type ContestRoleResponse} from "@/lib/permissions";

import type {ContestModel, UserModel} from "@/contracts/core/v1/schema";

export type ContestHeaderNavProps = {
  contest: ContestModel;
  contestRole?: ContestRoleResponse | null;
  orgLogin?: string;
  user?: UserModel | null;
};

export const ContestHeaderNav = ({
  contest,
  orgLogin,
  user: propUser,
}: ContestHeaderNavProps): ReactNode => {
  const {pathname} = useLocation();
  const {user: sessionUser} = useSession();
  const user = propUser !== undefined ? propUser : sessionUser;

  const org = orgLogin || contest.organization_login;
  const contestBase = `/${org}/${contest.login}`;

  const getActiveTabKey = (): string => {
    if (!pathname) {
      return "tasks";
    }
    if (pathname.startsWith(`${contestBase}/messages`)) {
      return "messages";
    }
    if (pathname.startsWith(`${contestBase}/submit`)) {
      return "submit";
    }
    if (pathname.startsWith(`${contestBase}/drafts`)) {
      return "drafts";
    }
    if (pathname.startsWith(`${contestBase}/mysubmissions`)) {
      return "mysubmissions";
    }
    if (pathname.startsWith(`${contestBase}/submissions`)) {
      return "allsubmissions";
    }
    if (pathname.startsWith(`${contestBase}/monitor`)) {
      return "monitor";
    }
    if (pathname.startsWith(`${contestBase}/settings`)) {
      return "settings";
    }
    return "tasks";
  };

  const activeTab = getActiveTabKey();
  const items: AdaptiveTabItem[] = [];

  items.push({
    key: "tasks",
    label: "Задачи",
    href: contestBase,
    icon: IconPuzzle,
    active: activeTab === "tasks",
  });

  items.push({
    key: "messages",
    label: "Сообщения",
    href: `${contestBase}/messages`,
    icon: IconMessageDots,
    active: activeTab === "messages",
  });

  if (contest.permissions.submit) {
    items.push({
      key: "submit",
      label: "Послать решение",
      href: `${contestBase}/submit`,
      icon: IconSend,
      active: activeTab === "submit",
    });
  }

  if (contest.permissions.view_problems && user?.id && contest.enable_drafts) {
    items.push({
      key: "drafts",
      label: "Черновики",
      href: `${contestBase}/drafts`,
      icon: IconFileCode,
      active: activeTab === "drafts",
    });
  }

  if (contest.permissions.list_own_submissions && user?.id) {
    items.push({
      key: "mysubmissions",
      label: "Мои посылки",
      href: `${contestBase}/mysubmissions?order=desc&userId=${user.id}`,
      icon: IconUser,
      active: activeTab === "mysubmissions",
    });
  }

  if (contest.permissions.list_submissions) {
    items.push({
      key: "allsubmissions",
      label: "Все посылки",
      href: `${contestBase}/submissions?order=desc`,
      icon: IconMail,
      active: activeTab === "allsubmissions",
    });
  }

  if (contest.permissions.view_scoreboard) {
    items.push({
      key: "monitor",
      label: "Монитор",
      href: `${contestBase}/monitor`,
      icon: IconDeviceDesktop,
      active: activeTab === "monitor",
    });
  }

  if (contest.permissions.manage) {
    items.push({
      key: "settings",
      label: "Настройки",
      href: `${contestBase}/settings`,
      icon: IconSettings,
      active: activeTab === "settings",
    });
  }

  return <AdaptiveTabs items={items} />;
};
