import Ajv from "ajv";
import {describe, expect, test} from "bun:test";

const contract = Bun.YAML.parse(await Bun.file(
  new URL("../../contracts/observer/v1/asyncapi.yaml", import.meta.url),
).text()) as object;
const ajv = new Ajv({strict: false, validateFormats: false});
ajv.addSchema(contract, "observer");
const submissions = ajv.compile({$ref: "observer#/components/schemas/SubmissionsMessage"});
const contests = ajv.compile({$ref: "observer#/components/schemas/ContestMessage"});
const id = "11111111-1111-4111-8111-111111111111";

describe("observer message contract", () => {
  test("accepts zero scores and nullable failed tests", () => {
    for (const failedTest of [null, 0, 3]) {
      expect(submissions({
        event_type: "submissions.completed",
        payload: {id, score: 0, state: 0, failed_test: failedTest},
      })).toBe(true);
    }
  });

  test("requires the payload belonging to the discriminator", () => {
    expect(submissions({event_type: "submissions.test_started", payload: {id}})).toBe(false);
    expect(submissions({event_type: "submissions.test_started", payload: {id, number: 1}})).toBe(true);
    expect(contests({event_type: "contest.announcement_deleted", payload: {id, contest_id: id}})).toBe(true);
    expect(contests({event_type: "contest.announcement_created", payload: {id, contest_id: id}})).toBe(false);
  });

  test("rejects unknown events, wrong field types, and missing payloads", () => {
    expect(submissions({event_type: "submissions.unknown", payload: {id}})).toBe(false);
    expect(submissions({event_type: "submissions.completed", payload: {id, score: "zero", failed_test: null}}))
      .toBe(false);
    expect(submissions({event_type: "submissions.completed", payload: {id}})).toBe(false);
    expect(submissions({event_type: "submissions.completed"})).toBe(false);
  });
});
