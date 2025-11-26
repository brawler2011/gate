// Copyright © 2024 Ory Corp
// SPDX-License-Identifier: Apache-2.0

import { Registration } from "@ory/elements-react/theme"
import { getRegistrationFlow } from "@/lib/ory-flows"
import config from "@/ory.config"

type PageProps = {
  searchParams: Promise<{ flow?: string }>;
};

export default async function RegistrationPage({ searchParams }: PageProps) {
  const flow = await getRegistrationFlow(searchParams)

  if (!flow) {
    return null
  }

  return (
    <Registration
      flow={flow}
      config={config}
      components={{
        Card: {},
      }}
    />
  )
}
