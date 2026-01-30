"use client";

import { Badge, Select, Text } from "@mantine/core";
import { useRouter, useSearchParams } from "next/navigation";
import { getRoleColor } from "@/lib/lib";

export function UsersRoleFilter() {
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
    router.push(`/admin${query ? `?${query}` : ""}`);
  };

  const getSelectedBadge = () => {
    if (!currentRole) return null;
    if (currentRole === "admin") {
      return (
        <Badge color="red" variant="filled">
          Admin
        </Badge>
      );
    }
    if (currentRole === "user") {
      return (
        <Badge color="gray" variant="filled">
          User
        </Badge>
      );
    }
    return null;
  };

  return (
    <Select
      placeholder="Фильтр по роли (опционально)..."
      value={currentRole}
      onChange={handleChange}
      data={[
        { value: "", label: "Все роли" },
        { value: "admin", label: "Admin" },
        { value: "user", label: "User" },
      ]}
      renderOption={({ option }) =>
        option.value === "" ? (
          <Text style={{ cursor: "pointer" }}>Все роли</Text>
        ) : (
          <Badge
            color={getRoleColor(option.value)}
            style={{ cursor: "pointer" }}
          >
            {option.label}
          </Badge>
        )
      }
      leftSection={getSelectedBadge()}
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
}

