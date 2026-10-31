import {Container} from "@mantine/core";

import {AdminUserEditContent} from "@/components/admin";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api} from "@/lib/api";
import {usePageQuery, PageQueryState} from "@/lib/pageQuery";
import {notFound, redirect} from "@/lib/routeErrors";

import type {ReactNode} from "react";

const loadAdminUserEditPage = async ({params}: Props) => {
  const {username} = params;
  let decoded = "";
  try {
    decoded = decodeURIComponent(username);
  } catch {
    notFound();
  }

  // Check admin rights
  const [meError, meData] = await api.getMe();
  if (meError || !meData?.user || meData.user.role !== "admin") {
    redirect("/auth/login?return_to=/admin/users/" + encodeURIComponent(decoded));
  }

  const [error, data] = await api.getUser(decoded);
  return {username, decoded, meError, meData, error, data};
};

type Props = {
  params: { username: string };
};

const AdminUserEditPage = ({params}: Props): ReactNode => {
  const routeQuery = usePageQuery(["frontend/app/admin/users/[username]/page.tsx", params], () =>
    loadAdminUserEditPage({params}),
  );
  if (!routeQuery.data) {
    return <PageQueryState query={routeQuery} />;
  }
  const {error, data} = routeQuery.data;
  if (error) {
    if (error.status === 404) {
      notFound();
    }
    return (
      <Container size="sm" py="lg">
        <ErrorDisplay error={error} />
      </Container>
    );
  }
  return <AdminUserEditContent user={data!.user} />;
};

export default AdminUserEditPage;
