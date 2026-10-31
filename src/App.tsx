import {lazy, Suspense} from "react";
import {createRoutesFromElements, Route, Outlet, useParams, ScrollRestoration} from "react-router-dom";

import NotFound from "@/app/not-found";
import {QueryState} from "@/components/shared/QueryState";
import {RouteError} from "@/components/shared/RouteError";

import type {ReactNode} from "react";

const UserProfilePage = lazy(() => import("@/app/user/[username]/page"));

const OrganizationOrProfile = (): ReactNode => {
  const {slug = ""} = useParams();
  if (slug.startsWith("@")) {
    return <UserProfilePage />;
  }
  return <Outlet />;
};
export const App = (): ReactNode => {
  return (
    <>
      <ScrollRestoration />
      <Suspense fallback={<QueryState />}>
        <Outlet />
      </Suspense>
    </>
  );
};

export const appRoutes = createRoutesFromElements(
  <Route element={<App />} errorElement={<RouteError />} hydrateFallbackElement={<QueryState />}>
    <Route path="/blog/:id" lazy={async () => {
      const page = await import("@/app/blog/[id]/page");
      return {Component: page.default, loader: page.loader};
    }} />
    <Route path="/blog" lazy={async () => {
      const page = await import("@/app/blog/page");
      return {Component: page.default};
    }} />
    <Route path="/notifications" lazy={async () => {
      const page = await import("@/app/notifications/page");
      return {Component: page.default, loader: page.loader};
    }} />
    <Route path="/orgs" lazy={async () => {
      const page = await import("@/app/orgs/page");
      return {Component: page.default, loader: page.loader};
    }} />
    <Route path="/" lazy={async () => {
      const page = await import("@/app/page");
      return {Component: page.default};
    }} />
    <Route path="/settings" lazy={async () => {
      const page = await import("@/app/settings/page");
      return {Component: page.default, loader: page.loader};
    }} />
    <Route path="/submissions/:submission_id" lazy={async () => {
      const page = await import("@/app/submissions/[submission_id]/page");
      return {Component: page.default, loader: page.loader};
    }} />
    <Route path="/user/:username/*" element={<NotFound />} />
    <Route path="/user" element={<NotFound />} />
    <Route path="/admin" lazy={async () => ({Component: (await import("@/app/admin/layout")).default})}>
      <Route path="blogs" lazy={async () => {
        const page = await import("@/app/admin/blogs/page");
        return {Component: page.default};
      }} />
      <Route path="contests" lazy={async () => {
        const page = await import("@/app/admin/contests/page");
        return {Component: page.default};
      }} />
      <Route path="orgs" lazy={async () => {
        const page = await import("@/app/admin/orgs/page");
        return {Component: page.default};
      }} />
      <Route index lazy={async () => {
        const page = await import("@/app/admin/page");
        return {Component: page.default};
      }} />
      <Route path="problems" lazy={async () => {
        const page = await import("@/app/admin/problems/page");
        return {Component: page.default};
      }} />
      <Route path="submissions" lazy={async () => {
        const page = await import("@/app/admin/submissions/page");
        return {Component: page.default};
      }} />
      <Route path="users/:username" lazy={async () => {
        const page = await import("@/app/admin/users/[username]/page");
        return {Component: page.default, loader: page.loader};
      }} />
      <Route path="users" lazy={async () => {
        const page = await import("@/app/admin/users/page");
        return {Component: page.default};
      }} />
    </Route>
    <Route path="/auth" lazy={async () => ({Component: (await import("@/app/auth/layout")).default})}>
      <Route path="confirm-email-change" lazy={async () => {
        const page = await import("@/app/auth/confirm-email-change/page");
        return {Component: page.default};
      }} />
      <Route path="forgot-password" lazy={async () => {
        const page = await import("@/app/auth/forgot-password/page");
        return {Component: page.default};
      }} />
      <Route path="login" lazy={async () => {
        const page = await import("@/app/auth/login/page");
        return {Component: page.default};
      }} />
      <Route path="registration" lazy={async () => {
        const page = await import("@/app/auth/registration/page");
        return {Component: page.default};
      }} />
      <Route path="reset-password" lazy={async () => {
        const page = await import("@/app/auth/reset-password/page");
        return {Component: page.default};
      }} />
      <Route path="verify-email" lazy={async () => {
        const page = await import("@/app/auth/verify-email/page");
        return {Component: page.default};
      }} />
    </Route>
    <Route path="/:slug" element={<OrganizationOrProfile />} loader={async (args) => {
      if (args.params.slug?.startsWith("@")) {
        if (new URL(args.request.url).pathname.split("/").filter(Boolean).length !== 1) {
          throw new Response("Страница не найдена", {status: 404});
        }
        const {loader} = await import("@/app/user/[username]/page");
        return loader({...args, params: {username: args.params.slug.slice(1)}});
      }
      return null;
    }}>
      <Route lazy={async () => ({Component: (await import("@/app/[slug]/(org)/layout")).default})}>
        <Route path="members" lazy={async () => {
          const page = await import("@/app/[slug]/(org)/members/page");
          return {Component: page.default, loader: page.loader};
        }} />
        <Route index lazy={async () => {
          const page = await import("@/app/[slug]/(org)/page");
          return {Component: page.default, loader: (args) => args.params.slug?.startsWith("@") ? null : page.loader(args)};
        }} />
        <Route path="problems" lazy={async () => {
          const page = await import("@/app/[slug]/(org)/problems/page");
          return {Component: page.default, loader: page.loader};
        }} />
        <Route path="settings" lazy={async () => {
          const page = await import("@/app/[slug]/(org)/settings/page");
          return {Component: page.default, loader: page.loader};
        }} />
        <Route path="teams/:team_id" lazy={async () => {
          const page = await import("@/app/[slug]/(org)/teams/[team_id]/page");
          return {Component: page.default, loader: page.loader};
        }} />
        <Route path="teams" lazy={async () => {
          const page = await import("@/app/[slug]/(org)/teams/page");
          return {Component: page.default, loader: page.loader};
        }} />
      </Route>
      <Route path=":contestLogin" lazy={async () => ({Component: (await import("@/app/[slug]/[contestLogin]/layout")).default})}>
        <Route path=":letter" lazy={async () => {
          const page = await import("@/app/[slug]/[contestLogin]/[letter]/page");
          return {Component: page.default};
        }} />
        <Route path="drafts" lazy={async () => {
          const page = await import("@/app/[slug]/[contestLogin]/drafts/page");
          return {Component: page.default, loader: page.loader};
        }} />
        <Route path="messages" lazy={async () => {
          const page = await import("@/app/[slug]/[contestLogin]/messages/page");
          return {Component: page.default, loader: page.loader};
        }} />
        <Route path="monitor" lazy={async () => {
          const page = await import("@/app/[slug]/[contestLogin]/monitor/page");
          return {Component: page.default, loader: page.loader};
        }} />
        <Route path="mysubmissions" lazy={async () => {
          const page = await import("@/app/[slug]/[contestLogin]/mysubmissions/page");
          return {Component: page.default, loader: page.loader};
        }} />
        <Route index lazy={async () => {
          const page = await import("@/app/[slug]/[contestLogin]/page");
          return {Component: page.default};
        }} />
        <Route path="settings" lazy={async () => {
          const page = await import("@/app/[slug]/[contestLogin]/settings/page");
          return {Component: page.default, loader: page.loader};
        }} />
        <Route path="submissions" lazy={async () => {
          const page = await import("@/app/[slug]/[contestLogin]/submissions/page");
          return {Component: page.default, loader: page.loader};
        }} />
        <Route path="submit" lazy={async () => {
          const page = await import("@/app/[slug]/[contestLogin]/submit/page");
          return {Component: page.default};
        }} />
      </Route>
      <Route path="problems/:problem_id" lazy={async () => ({Component: (await import("@/app/[slug]/problems/[problem_id]/layout")).default})}>
        <Route path="access" lazy={async () => {
          const page = await import("@/app/[slug]/problems/[problem_id]/access/page");
          return {Component: page.default};
        }} />
        <Route path="checkers" lazy={async () => {
          const page = await import("@/app/[slug]/problems/[problem_id]/checkers/page");
          return {Component: page.default};
        }} />
        <Route path="generators" lazy={async () => ({Component: (await import("@/app/[slug]/problems/[problem_id]/generators/page")).default})} />
        <Route path="import" lazy={async () => {
          const page = await import("@/app/[slug]/problems/[problem_id]/import/page");
          return {Component: page.default};
        }} />
        <Route path="interactors" lazy={async () => ({Component: (await import("@/app/[slug]/problems/[problem_id]/interactors/page")).default})} />
        <Route path="lib" lazy={async () => {
          const page = await import("@/app/[slug]/problems/[problem_id]/lib/page");
          return {Component: page.default};
        }} />
        <Route path="media" lazy={async () => {
          const page = await import("@/app/[slug]/problems/[problem_id]/media/page");
          return {Component: page.default};
        }} />
        <Route path="packages" lazy={async () => {
          const page = await import("@/app/[slug]/problems/[problem_id]/packages/page");
          return {Component: page.default};
        }} />
        <Route index lazy={async () => {
          const page = await import("@/app/[slug]/problems/[problem_id]/page");
          return {Component: page.default};
        }} />
        <Route path="solutions" lazy={async () => ({Component: (await import("@/app/[slug]/problems/[problem_id]/solutions/page")).default})} />
        <Route path="statement" lazy={async () => ({Component: (await import("@/app/[slug]/problems/[problem_id]/statement/page")).default})} />
        <Route path="tests" lazy={async () => {
          const page = await import("@/app/[slug]/problems/[problem_id]/tests/page");
          return {Component: page.default};
        }} />
        <Route path="validators" lazy={async () => ({Component: (await import("@/app/[slug]/problems/[problem_id]/validators/page")).default})} />
      </Route>
    </Route>
    <Route path="*" element={<NotFound />} />
  </Route>,
);
