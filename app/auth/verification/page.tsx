// Copyright © 2024 Ory Corp
// SPDX-License-Identifier: Apache-2.0

import CustomCardHeader from "@/components/custom-card-header"
import { Verification } from "@ory/elements-react/theme"
import { getVerificationFlow } from "@/lib/ory-flows"
import config from "@/ory.config"

type PageProps = {
  searchParams: Promise<{ flow?: string }>;
};

export default async function VerificationPage({ searchParams }: PageProps) {
  const flow = await getVerificationFlow(searchParams)

  if (!flow) {
    return null
  }

  return (
    <Verification
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
