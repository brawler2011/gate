import {useQueryClient} from "@tanstack/react-query";
import {useEffect, useState} from "react";

import {useSession} from "@/contexts/SessionContext";
import {api} from "@/lib/api";

import type {VirtualRunModel} from "@/contracts/core/v1/schema";

// useVirtualRun returns the caller's virtual run and refreshes contest data the moment it ends.
// pending: the lookup is in flight; known: it succeeded, so replaying can be trusted.
type VirtualRunState = {run: VirtualRunModel | null; replaying: boolean; pending: boolean; known: boolean};

export const useVirtualRun = (orgLogin: string, contestLogin: string): VirtualRunState => {
  const {user} = useSession();
  const client = useQueryClient();
  const query = api.useQuery("get", "/organizations/{org_login}/contests/{contest_login}/virtual", {
    params: {path: {org_login: orgLogin, contest_login: contestLogin}},
  }, {enabled: Boolean(user)});
  const run = query.data?.virtual_run ?? null;
  const [now, setNow] = useState(() => Date.now());
  const endsAt = run ? Date.parse(run.ends_at) : 0;
  useEffect(() => {
    if (endsAt <= Date.now()) {
      return;
    }
    // Browser timers cannot represent delays beyond a signed 32-bit interval.
    const timer = setTimeout(() => {
      setNow(Date.now());
      void client.invalidateQueries();
    }, Math.min(endsAt - Date.now() + 1, 2147483647));
    return () => clearTimeout(timer);
  }, [client, endsAt]);
  return {
    run, replaying: run !== null && endsAt > now,
    pending: Boolean(user) && query.isPending, known: !user || query.isSuccess,
  };
};
