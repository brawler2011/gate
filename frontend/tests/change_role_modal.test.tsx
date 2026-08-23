import "./setup-dom";

import {afterAll, beforeAll, describe, expect, it} from "bun:test";
import React from "react";

import {ChangeRoleModal} from "@/components/shared/ChangeRoleModal";

import {renderWithProviders, setupDOMEnvironment} from "./test-utils";

describe("ChangeRoleModal Component test", () => {
  let cleanupDOM: () => void;

  beforeAll(() => {
    cleanupDOM = setupDOMEnvironment();
  });

  afterAll(() => {
    cleanupDOM();
  });

  it("renders participant username and role selector in modal portal", () => {
    renderWithProviders(
      <ChangeRoleModal
        opened
        onClose={() => {}}
        participant={{username: "alex_coder", userId: "u-1"}}
        currentRole="participant"
        onSubmit={async () => {}}
      />
    );

    expect(document.body.textContent).toContain("Изменить роль");
    expect(document.body.textContent).toContain("alex_coder");
    expect(document.body.textContent).toContain("Участник");
    expect(document.body.textContent).toContain("Применить");
  });
});
