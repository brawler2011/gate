import "./setup-dom";

import {afterAll, beforeAll, describe, expect, it} from "bun:test";
import React from "react";

import {EntityManagementTable} from "@/components/shared/EntityManagementTable";

import {renderWithProviders, setupDOMEnvironment} from "./test-utils";

describe("EntityManagementTable Component test", () => {
  let cleanupDOM: () => void;

  beforeAll(() => {
    cleanupDOM = setupDOMEnvironment();
  });

  afterAll(() => {
    cleanupDOM();
  });

  interface TestItem {
    id: string;
    name: string;
    role: string;
  }

  const sampleItems: TestItem[] = [
    {id: "1", name: "Team One", role: "participant"},
    {id: "2", name: "Team Two", role: "moderator"},
  ];

  it("renders empty message when no items", () => {
    const {container} = renderWithProviders(
      <EntityManagementTable<TestItem>
        items={[]}
        getItemKey={(item) => item.id}
        emptyMessage="Нет данных"
      />
    );
    expect(container.textContent).toContain("Нет данных");
  });

  it("renders rows and role badges correctly", () => {
    const {container} = renderWithProviders(
      <EntityManagementTable<TestItem>
        items={sampleItems}
        getItemKey={(item) => item.id}
        getTitle={(item) => item.name}
        getRoleBadge={(item) => ({
          label: item.role === "participant" ? "Участник" : "Модератор",
          color: item.role === "participant" ? "blue" : "yellow",
        })}
      />
    );

    expect(container.textContent).toContain("Team One");
    expect(container.textContent).toContain("Team Two");
    expect(container.textContent).toContain("Участник");
    expect(container.textContent).toContain("Модератор");
  });
});
