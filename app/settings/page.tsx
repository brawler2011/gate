import { Settings } from "@ory/elements-react/theme"
import { SessionProvider } from "@ory/elements-react/client"
import { getSettingsFlow } from "@/lib/ory-flows"
import "@ory/elements-react/theme/styles.css"
import config from "@/ory.config"

type PageProps = {
  searchParams: Promise<{ flow?: string }>;
};

export default async function SettingsPage({ searchParams }: PageProps) {
  const flow = await getSettingsFlow(searchParams)

  if (!flow) {
    return null
  }

  return (
    <div className="flex flex-col gap-8 items-center mb-8">
      <SessionProvider>
        <Settings
          flow={flow}
          config={config}
          components={{
            Card: {},
          }}
        />
      </SessionProvider>
    </div>
  )
}
