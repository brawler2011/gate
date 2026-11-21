import {expect, it} from "bun:test";

import {problemMessage} from "../lib/problemText";

const problem = (code: string, extra: object = {}) => ({type: "about:blank", title: "x", status: 400, code, request_id: "r", detail: "English detail", ...extra});

it("prefers the action's fallback over vague codes and keeps specific codes", () => {
  expect(problemMessage(problem("conflict"), "Не удалось добавить участника")).toBe("Не удалось добавить участника");
  expect(problemMessage(problem("already_member"), "Не удалось добавить участника")).toBe("Уже является участником");
  expect(problemMessage(problem("forbidden", {status: 403}), "Не удалось удалить")).toBe("Недостаточно прав");
  expect(problemMessage(undefined, "Не удалось удалить")).toBe("Не удалось удалить");
});

it("renders field errors with their labels and bounds", () => {
  const errors = [
    {pointer: "/prefix", code: "length", params: {max: 21}},
    {pointer: "/password", code: "length", params: {min: 8}},
    {pointer: "/unknown", code: "required"},
  ];
  expect(problemMessage(problem("invalid", {errors}), "Не удалось")).toBe(
    "Префикс: не более 21 символа. Пароль: не менее 8 символов. Обязательное поле",
  );
});
