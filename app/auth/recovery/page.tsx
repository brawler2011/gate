// Copyright © 2024 Ory Corp
// SPDX-License-Identifier: Apache-2.0

import { Recovery } from "@ory/elements-react/theme"
import { getRecoveryFlow } from "@/lib/ory-flows"
import CustomCardHeader from "@/components/custom-card-header"
import config from "@/ory.config"

type PageProps = {
  searchParams: Promise<{ flow?: string }>;
};

export default async function RecoveryPage({ searchParams }: PageProps) {
  const flow = await getRecoveryFlow(searchParams)

  if (!flow) {
    return null
  }

  return (
    <Recovery
      flow={flow}
      config={config}
      components={{
        Card: {
          Header: CustomCardHeader,
        },
      }}
    />
  )
}
