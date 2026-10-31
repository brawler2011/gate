import {createElement, useEffect, lazy, Suspense} from "react";
import {
  createRoutesFromElements,
  Route,
  Outlet,
  useParams,
  useSearchParams,
  useLocation,
  useRoutes,
} from "react-router-dom";

import OrgLayout from "@/app/[slug]/(org)/layout";
import ContestLayout from "@/app/[slug]/[contestLogin]/layout";
import ProblemLayout from "@/app/[slug]/problems/[problem_id]/layout";
import AdminLayout from "@/app/admin/layout";
import AuthLayout from "@/app/auth/layout";
import NotFound from "@/app/not-found";
import {RouteBoundary} from "@/components/shared/RouteBoundary";

import type {ComponentType, ReactNode} from "react";

const OrgMembersPage = lazy(() => import("@/app/[slug]/(org)/members/page"));
const OrgPage = lazy(() => import("@/app/[slug]/(org)/page"));
const OrgProblemsPage = lazy(() => import("@/app/[slug]/(org)/problems/page"));
const OrgSettingsPage = lazy(() => import("@/app/[slug]/(org)/settings/page"));
const OrgTeamsTeamPage = lazy(() => import("@/app/[slug]/(org)/teams/[team_id]/page"));
const OrgTeamsPage = lazy(() => import("@/app/[slug]/(org)/teams/page"));
const OrgContestTaskPage = lazy(() => import("@/app/[slug]/[contestLogin]/[letter]/page"));
const OrgContestDraftsPage = lazy(() => import("@/app/[slug]/[contestLogin]/drafts/page"));
const OrgContestMessagesPage = lazy(() => import("@/app/[slug]/[contestLogin]/messages/page"));
const OrgContestMonitorPage = lazy(() => import("@/app/[slug]/[contestLogin]/monitor/page"));
const OrgContestMysubmissionsPage = lazy(() => import("@/app/[slug]/[contestLogin]/mysubmissions/page"));
const OrgContestPage = lazy(() => import("@/app/[slug]/[contestLogin]/page"));
const OrgContestSettingsPage = lazy(() => import("@/app/[slug]/[contestLogin]/settings/page"));
const OrgContestSubmissionsPage = lazy(() => import("@/app/[slug]/[contestLogin]/submissions/page"));
const OrgContestSubmitPage = lazy(() => import("@/app/[slug]/[contestLogin]/submit/page"));
const OrgProblemsProblemAccessPage = lazy(() => import("@/app/[slug]/problems/[problem_id]/access/page"));
const OrgProblemsProblemCheckersPage = lazy(() => import("@/app/[slug]/problems/[problem_id]/checkers/page"));
const OrgProblemsProblemGeneratorsPage = lazy(
  () => import("@/app/[slug]/problems/[problem_id]/generators/page"),
);
const OrgProblemsProblemImportPage = lazy(() => import("@/app/[slug]/problems/[problem_id]/import/page"));
const OrgProblemsProblemInteractorsPage = lazy(
  () => import("@/app/[slug]/problems/[problem_id]/interactors/page"),
);
const OrgProblemsProblemLibPage = lazy(() => import("@/app/[slug]/problems/[problem_id]/lib/page"));
const OrgProblemsProblemMediaPage = lazy(() => import("@/app/[slug]/problems/[problem_id]/media/page"));
const OrgProblemsProblemPackagesPage = lazy(() => import("@/app/[slug]/problems/[problem_id]/packages/page"));
const OrgProblemsProblemPage = lazy(() => import("@/app/[slug]/problems/[problem_id]/page"));
const OrgProblemsProblemSolutionsPage = lazy(
  () => import("@/app/[slug]/problems/[problem_id]/solutions/page"),
);
const OrgProblemsProblemStatementPage = lazy(
  () => import("@/app/[slug]/problems/[problem_id]/statement/page"),
);
const OrgProblemsProblemTestsPage = lazy(() => import("@/app/[slug]/problems/[problem_id]/tests/page"));
const OrgProblemsProblemValidatorsPage = lazy(
  () => import("@/app/[slug]/problems/[problem_id]/validators/page"),
);
const AdminBlogsPage = lazy(() => import("@/app/admin/blogs/page"));
const AdminContestsPage = lazy(() => import("@/app/admin/contests/page"));
const AdminOrgsPage = lazy(() => import("@/app/admin/orgs/page"));
const AdminPage = lazy(() => import("@/app/admin/page"));
const AdminProblemsPage = lazy(() => import("@/app/admin/problems/page"));
const AdminSubmissionsPage = lazy(() => import("@/app/admin/submissions/page"));
const AdminUsersProfilePage = lazy(() => import("@/app/admin/users/[username]/page"));
const AdminUsersPage = lazy(() => import("@/app/admin/users/page"));
const AuthConfirmEmailChangePage = lazy(() => import("@/app/auth/confirm-email-change/page"));
const AuthForgotPasswordPage = lazy(() => import("@/app/auth/forgot-password/page"));
const AuthLoginPage = lazy(() => import("@/app/auth/login/page"));
const AuthRegistrationPage = lazy(() => import("@/app/auth/registration/page"));
const AuthResetPasswordPage = lazy(() => import("@/app/auth/reset-password/page"));
const AuthVerifyEmailPage = lazy(() => import("@/app/auth/verify-email/page"));
const BlogPostPage = lazy(() => import("@/app/blog/[id]/page"));
const BlogPage = lazy(() => import("@/app/blog/page"));
const NotificationsPage = lazy(() => import("@/app/notifications/page"));
const OrgsPage = lazy(() => import("@/app/orgs/page"));
const HomePage = lazy(() => import("@/app/page"));
const SettingsPage = lazy(() => import("@/app/settings/page"));
const SubmissionsSubmissionPage = lazy(() => import("@/app/submissions/[submission_id]/page"));
const UserProfilePage = lazy(() => import("@/app/user/[username]/page"));

const PageRoute = <P extends object>({component}: { component: ComponentType<P> }): ReactNode => {
  const params = useParams();
  const [search] = useSearchParams();
  return createElement(component, {params, searchParams: Object.fromEntries(search)} as P);
};
const OrganizationOrProfile = (): ReactNode => {
  const {slug = ""} = useParams();
  const {pathname} = useLocation();
  const [search] = useSearchParams();
  if (slug.startsWith("@")) {
    if (pathname.split("/").filter(Boolean).length !== 1) {
      return <NotFound />;
    }
    return <UserProfilePage params={{username: slug.slice(1)}} searchParams={Object.fromEntries(search)} />;
  }
  return <Outlet />;
};
export const appRoutes = createRoutesFromElements(
  <>
    <Route path="/blog/:id" element={<PageRoute component={BlogPostPage} />} />
    <Route path="/blog" element={<PageRoute component={BlogPage} />} />
    <Route path="/notifications" element={<PageRoute component={NotificationsPage} />} />
    <Route path="/orgs" element={<PageRoute component={OrgsPage} />} />
    <Route path="/" element={<PageRoute component={HomePage} />} />
    <Route path="/settings" element={<PageRoute component={SettingsPage} />} />
    <Route path="/submissions/:submission_id" element={<PageRoute component={SubmissionsSubmissionPage} />} />
    <Route path="/user/:username/*" element={<NotFound />} />
    <Route path="/user" element={<NotFound />} />
    <Route path="/admin" element={<AdminLayout />}>
      <Route path="blogs" element={<PageRoute component={AdminBlogsPage} />} />
      <Route path="contests" element={<PageRoute component={AdminContestsPage} />} />
      <Route path="orgs" element={<PageRoute component={AdminOrgsPage} />} />
      <Route index element={<PageRoute component={AdminPage} />} />
      <Route path="problems" element={<PageRoute component={AdminProblemsPage} />} />
      <Route path="submissions" element={<PageRoute component={AdminSubmissionsPage} />} />
      <Route path="users/:username" element={<PageRoute component={AdminUsersProfilePage} />} />
      <Route path="users" element={<PageRoute component={AdminUsersPage} />} />
    </Route>
    <Route path="/auth" element={<AuthLayout />}>
      <Route path="confirm-email-change" element={<PageRoute component={AuthConfirmEmailChangePage} />} />
      <Route path="forgot-password" element={<PageRoute component={AuthForgotPasswordPage} />} />
      <Route path="login" element={<PageRoute component={AuthLoginPage} />} />
      <Route path="registration" element={<PageRoute component={AuthRegistrationPage} />} />
      <Route path="reset-password" element={<PageRoute component={AuthResetPasswordPage} />} />
      <Route path="verify-email" element={<PageRoute component={AuthVerifyEmailPage} />} />
    </Route>
    <Route path="/:slug" element={<OrganizationOrProfile />}>
      <Route element={<OrgLayout />}>
        <Route path="members" element={<PageRoute component={OrgMembersPage} />} />
        <Route index element={<PageRoute component={OrgPage} />} />
        <Route path="problems" element={<PageRoute component={OrgProblemsPage} />} />
        <Route path="settings" element={<PageRoute component={OrgSettingsPage} />} />
        <Route path="teams/:team_id" element={<PageRoute component={OrgTeamsTeamPage} />} />
        <Route path="teams" element={<PageRoute component={OrgTeamsPage} />} />
      </Route>
      <Route path=":contestLogin" element={<ContestLayout />}>
        <Route path=":letter" element={<PageRoute component={OrgContestTaskPage} />} />
        <Route path="drafts" element={<PageRoute component={OrgContestDraftsPage} />} />
        <Route path="messages" element={<PageRoute component={OrgContestMessagesPage} />} />
        <Route path="monitor" element={<PageRoute component={OrgContestMonitorPage} />} />
        <Route path="mysubmissions" element={<PageRoute component={OrgContestMysubmissionsPage} />} />
        <Route index element={<PageRoute component={OrgContestPage} />} />
        <Route path="settings" element={<PageRoute component={OrgContestSettingsPage} />} />
        <Route path="submissions" element={<PageRoute component={OrgContestSubmissionsPage} />} />
        <Route path="submit" element={<PageRoute component={OrgContestSubmitPage} />} />
      </Route>
      <Route path="problems/:problem_id" element={<ProblemLayout />}>
        <Route path="access" element={<PageRoute component={OrgProblemsProblemAccessPage} />} />
        <Route path="checkers" element={<PageRoute component={OrgProblemsProblemCheckersPage} />} />
        <Route path="generators" element={<PageRoute component={OrgProblemsProblemGeneratorsPage} />} />
        <Route path="import" element={<PageRoute component={OrgProblemsProblemImportPage} />} />
        <Route path="interactors" element={<PageRoute component={OrgProblemsProblemInteractorsPage} />} />
        <Route path="lib" element={<PageRoute component={OrgProblemsProblemLibPage} />} />
        <Route path="media" element={<PageRoute component={OrgProblemsProblemMediaPage} />} />
        <Route path="packages" element={<PageRoute component={OrgProblemsProblemPackagesPage} />} />
        <Route index element={<PageRoute component={OrgProblemsProblemPage} />} />
        <Route path="solutions" element={<PageRoute component={OrgProblemsProblemSolutionsPage} />} />
        <Route path="statement" element={<PageRoute component={OrgProblemsProblemStatementPage} />} />
        <Route path="tests" element={<PageRoute component={OrgProblemsProblemTestsPage} />} />
        <Route path="validators" element={<PageRoute component={OrgProblemsProblemValidatorsPage} />} />
      </Route>
    </Route>
    <Route path="*" element={<NotFound />} />
  </>,
);
const RouteTree = (): ReactNode => useRoutes(appRoutes);
const ScrollToTop = (): null => {
  const {pathname} = useLocation();
  useEffect(() => {
    window.scrollTo(0, 0);
  }, [pathname]);
  return null;
};
export const App = (): ReactNode => (
  <>
    <ScrollToTop />
    <RouteBoundary>
      <Suspense fallback={<div role="status">Загрузка…</div>}>
        <RouteTree />
      </Suspense>
    </RouteBoundary>
  </>
);
