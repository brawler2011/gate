"use client";

import {Badge, Select, Text} from "@mantine/core";
import {useRouter, useSearchParams} from "next/navigation";

import type {ReactNode} from "react";

const ROLE_OPTIONS = [
  {value: "admin", label: "Admin", color: "red"},
  {value: "user", label: "User", color: "gray"},
] as const;

export const UsersRoleFilter = (): ReactNode => {
  const router = useRouter();
  const searchParams = useSearchParams();
  const currentRole = searchParams.get("role") || "";

  const handleChange = (value: string | null) => {
    // Игнорируем null (клик на уже выбранную опцию)
    if (value === null) {
      return;
    }

    const params = new URLSearchParams(searchParams);
    if (value) {
      params.set("role", value);
    } else {
      params.delete("role");
    }
    params.delete("page"); // Reset to first page on filter change

    const query = params.toString();
    router.push(`/admin/users${query ? `?${query}` : ""}`);
  };

  const selectedRole = ROLE_OPTIONS.find((r) => r.value === currentRole);

  return (
    <Select
      placeholder="Фильтр по роли (опционально)..."
      value={currentRole}
      onChange={handleChange}
      data={[
        {value: "", label: "Все роли"},
        ...ROLE_OPTIONS,
      ]}
      renderOption={({option}) => {
        const role = ROLE_OPTIONS.find((r) => r.value === option.value);
        if (!role) {
          return <Text style={{cursor: "pointer"}}>{option.label}</Text>;
        }
        return (
          <Badge color={role.color} style={{cursor: "pointer"}}>
            {role.label}
          </Badge>
        );
      }}
      leftSection={
        selectedRole ? (
          <Badge color={selectedRole.color} variant="filled">
            {selectedRole.label}
          </Badge>
        ) : null
      }
      leftSectionWidth={85}
      styles={{
        input: {
          color: currentRole && currentRole !== "" ? "transparent" : undefined,
        },
        section: {
          pointerEvents: "none",
        },
      }}
    />
  );
};
