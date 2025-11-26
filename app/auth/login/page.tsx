import { Login } from "@ory/elements-react/theme"
import { getLoginFlow } from "@/lib/ory-flows"
import config from "@/ory.config"

type PageProps = {
  searchParams: Promise<{ flow?: string }>;
};

export default async function LoginPage({ searchParams }: PageProps) {
  const flow = await getLoginFlow(searchParams)

  if (!flow) {
    return null
  }

  return (
    <Login
      flow={flow}
      config={config}
    />
  )
}
