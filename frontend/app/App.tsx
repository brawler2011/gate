import {lazy, Suspense} from "react";
import {createRoutesFromElements, Route, Outlet, useParams, ScrollRestoration} from "react-router-dom";

import {QueryState} from "@/components/shared/QueryState";
import {RouteError} from "@/components/shared/RouteError";
import NotFound from "@/pages/NotFoundPage";

import type {ReactNode} from "react";

const UserProfilePage = lazy(() => import("@/pages/users/UserPage"));

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
      const page = await import("@/pages/blog/post/BlogPostPage");
      return {Component: page.default, loader: page.loader};
    }} />
    <Route path="/blog" lazy={async () => {
      const page = await import("@/pages/blog/BlogPage");
      return {Component: page.default};
    }} />
    <Route path="/notifications" lazy={async () => {
      const page = await import("@/pages/notifications/NotificationsPage");
      return {Component: page.default, loader: page.loader};
    }} />
    <Route path="/orgs" lazy={async () => {
      const page = await import("@/pages/orgs/OrgsPage");
      return {Component: page.default, loader: page.loader};
    }} />
    <Route path="/" lazy={async () => {
      const page = await import("@/pages/home/HomePage");
      return {Component: page.default};
    }} />
    <Route path="/settings" lazy={async () => {
      const page = await import("@/pages/settings/SettingsPage");
      return {Component: page.default, loader: page.loader};
    }} />
    <Route path="/submissions/:submission_id" lazy={async () => {
      const page = await import("@/pages/submissions/SubmissionPage");
      return {Component: page.default, loader: page.loader};
    }} />
    <Route path="/user/:username/*" element={<NotFound />} />
    <Route path="/user" element={<NotFound />} />
    <Route path="/admin" lazy={async () => ({Component: (await import("@/pages/admin/AdminLayout")).default})}>
      <Route path="blogs" lazy={async () => {
        const page = await import("@/pages/admin/blogs/BlogsPage");
        return {Component: page.default};
      }} />
      <Route path="contests" lazy={async () => {
        const page = await import("@/pages/admin/contests/ContestsPage");
        return {Component: page.default};
      }} />
      <Route path="orgs" lazy={async () => {
        const page = await import("@/pages/admin/orgs/OrgsPage");
        return {Component: page.default};
      }} />
      <Route index lazy={async () => {
        const page = await import("@/pages/admin/AdminPage");
        return {Component: page.default};
      }} />
      <Route path="problems" lazy={async () => {
        const page = await import("@/pages/admin/problems/ProblemsPage");
        return {Component: page.default};
      }} />
      <Route path="submissions" lazy={async () => {
        const page = await import("@/pages/admin/submissions/SubmissionsPage");
        return {Component: page.default};
      }} />
      <Route path="users/:username" lazy={async () => {
        const page = await import("@/pages/admin/users/detail/AdminUserPage");
        return {Component: page.default, loader: page.loader};
      }} />
      <Route path="users" lazy={async () => {
        const page = await import("@/pages/admin/users/UsersPage");
        return {Component: page.default};
      }} />
    </Route>
    <Route path="/auth" lazy={async () => ({Component: (await import("@/pages/auth/AuthLayout")).default})}>
      <Route path="confirm-email-change" lazy={async () => {
        const page = await import("@/pages/auth/confirm-email-change/ConfirmEmailChangePage");
        return {Component: page.default};
      }} />
      <Route path="forgot-password" lazy={async () => {
        const page = await import("@/pages/auth/forgot-password/ForgotPasswordPage");
        return {Component: page.default};
      }} />
      <Route path="login" lazy={async () => {
        const page = await import("@/pages/auth/login/LoginPage");
        return {Component: page.default};
      }} />
      <Route path="registration" lazy={async () => {
        const page = await import("@/pages/auth/registration/RegistrationPage");
        return {Component: page.default};
      }} />
      <Route path="reset-password" lazy={async () => {
        const page = await import("@/pages/auth/reset-password/ResetPasswordPage");
        return {Component: page.default};
      }} />
      <Route path="verify-email" lazy={async () => {
        const page = await import("@/pages/auth/verify-email/VerifyEmailPage");
        return {Component: page.default};
      }} />
    </Route>
    <Route path="/:slug" element={<OrganizationOrProfile />} loader={async (args) => {
      if (args.params.slug?.startsWith("@")) {
        if (new URL(args.request.url).pathname.split("/").filter(Boolean).length !== 1) {
          throw new Response("Страница не найдена", {status: 404});
        }
        const {loader} = await import("@/pages/users/UserPage");
        return loader({...args, params: {username: args.params.slug.slice(1)}});
      }
      return null;
    }}>
      <Route lazy={async () => ({Component: (await import("@/pages/orgs/OrganizationLayout")).default})}>
        <Route path="members" lazy={async () => {
          const page = await import("@/pages/orgs/members/MembersPage");
          return {Component: page.default, loader: page.loader};
        }} />
        <Route index lazy={async () => {
          const page = await import("@/pages/orgs/OrganizationPage");
          return {Component: page.default, loader: (args) => args.params.slug?.startsWith("@") ? null : page.loader(args)};
        }} />
        <Route path="problems" lazy={async () => {
          const page = await import("@/pages/orgs/problems/ProblemsPage");
          return {Component: page.default, loader: page.loader};
        }} />
        <Route path="settings" lazy={async () => {
          const page = await import("@/pages/orgs/settings/SettingsPage");
          return {Component: page.default, loader: page.loader};
        }} />
        <Route path="teams/:team_id" lazy={async () => {
          const page = await import("@/pages/orgs/teams/detail/TeamPage");
          return {Component: page.default, loader: page.loader};
        }} />
        <Route path="teams" lazy={async () => {
          const page = await import("@/pages/orgs/teams/TeamsPage");
          return {Component: page.default, loader: page.loader};
        }} />
      </Route>
      <Route path=":contestLogin" lazy={async () => ({Component: (await import("@/pages/contests/ContestLayout")).default})}>
        <Route path=":letter" lazy={async () => {
          const page = await import("@/pages/contests/task/TaskPage");
          return {Component: page.default};
        }} />
        <Route path="drafts" lazy={async () => {
          const page = await import("@/pages/contests/drafts/DraftsPage");
          return {Component: page.default, loader: page.loader};
        }} />
        <Route path="messages" lazy={async () => {
          const page = await import("@/pages/contests/messages/MessagesPage");
          return {Component: page.default, loader: page.loader};
        }} />
        <Route path="monitor" lazy={async () => {
          const page = await import("@/pages/contests/monitor/MonitorPage");
          return {Component: page.default, loader: page.loader};
        }} />
        <Route path="mysubmissions" lazy={async () => {
          const page = await import("@/pages/contests/mysubmissions/MysubmissionsPage");
          return {Component: page.default, loader: page.loader};
        }} />
        <Route index lazy={async () => {
          const page = await import("@/pages/contests/ContestPage");
          return {Component: page.default};
        }} />
        <Route path="settings" lazy={async () => {
          const page = await import("@/pages/contests/settings/SettingsPage");
          return {Component: page.default, loader: page.loader};
        }} />
        <Route path="submissions" lazy={async () => {
          const page = await import("@/pages/contests/submissions/SubmissionsPage");
          return {Component: page.default, loader: page.loader};
        }} />
        <Route path="submit" lazy={async () => {
          const page = await import("@/pages/contests/submit/SubmitPage");
          return {Component: page.default};
        }} />
      </Route>
      <Route path="problems/:problem_id" lazy={async () => ({Component: (await import("@/pages/problems/ProblemLayout")).default})}>
        <Route path="access" lazy={async () => {
          const page = await import("@/pages/problems/access/AccessPage");
          return {Component: page.default};
        }} />
        <Route path="checkers" lazy={async () => {
          const page = await import("@/pages/problems/checkers/CheckersPage");
          return {Component: page.default};
        }} />
        <Route path="generators" lazy={async () => ({Component: (await import("@/pages/problems/generators/GeneratorsPage")).default})} />
        <Route path="import" lazy={async () => {
          const page = await import("@/pages/problems/import/ImportPage");
          return {Component: page.default};
        }} />
        <Route path="interactors" lazy={async () => ({Component: (await import("@/pages/problems/interactors/InteractorsPage")).default})} />
        <Route path="lib" lazy={async () => {
          const page = await import("@/pages/problems/lib/LibPage");
          return {Component: page.default};
        }} />
        <Route path="media" lazy={async () => {
          const page = await import("@/pages/problems/media/MediaPage");
          return {Component: page.default};
        }} />
        <Route path="packages" lazy={async () => {
          const page = await import("@/pages/problems/packages/PackagesPage");
          return {Component: page.default};
        }} />
        <Route index lazy={async () => {
          const page = await import("@/pages/problems/ProblemGeneralPage");
          return {Component: page.default};
        }} />
        <Route path="solutions" lazy={async () => ({Component: (await import("@/pages/problems/solutions/SolutionsPage")).default})} />
        <Route path="statement" lazy={async () => ({Component: (await import("@/pages/problems/statement/StatementPage")).default})} />
        <Route path="tests" lazy={async () => {
          const page = await import("@/pages/problems/tests/TestsPage");
          return {Component: page.default};
        }} />
        <Route path="validators" lazy={async () => ({Component: (await import("@/pages/problems/validators/ValidatorsPage")).default})} />
      </Route>
    </Route>
    <Route path="*" element={<NotFound />} />
  </Route>,
);
