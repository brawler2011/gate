"use client";

import { Select } from '@mantine/core';
import { useRouter, useSearchParams } from 'next/navigation';
import type { OrganizationModel } from '@contracts/gateway/v1';

type Props = { orgs: OrganizationModel[]; selectedOrgId?: string };

export function WorkshopOrgSelector({ orgs, selectedOrgId }: Props) {
  const router = useRouter();
  const searchParams = useSearchParams();

  if (orgs.length === 0) return null;

  const data = [
    { value: '', label: 'Все организации' },
    ...orgs.map((o) => ({ value: o.id, label: o.name })),
  ];

  const handleChange = (value: string | null) => {
    const params = new URLSearchParams(searchParams.toString());
    if (value) {
      params.set('org_id', value);
    } else {
      params.delete('org_id');
    }
    params.delete('page');
    router.push(`/workshop?${params.toString()}`);
  };

  return (
    <Select
      placeholder="Организация"
      data={data}
      value={selectedOrgId ?? ''}
      onChange={handleChange}
      clearable
      w={200}
    />
  );
}
