import Ajv from "ajv";
import {describe, expect, test} from "bun:test";

const contract = Bun.YAML.parse(await Bun.file(
  new URL("../../contracts/observer/v1/asyncapi.yaml", import.meta.url),
).text()) as object;
const ajv = new Ajv({strict: false, validateFormats: false});
ajv.addSchema(Bun.YAML.parse(await Bun.file(new URL("../../contracts/core/v1/openapi.yaml", import.meta.url)).text()) as object,
  "../../core/v1/openapi.yaml");
ajv.addSchema(contract, "observer");
const submissions = ajv.compile({$ref: "observer#/components/schemas/SubmissionsMessage"});
const contests = ajv.compile({$ref: "observer#/components/schemas/ContestMessage"});
const id = "11111111-1111-4111-8111-111111111111";

describe("observer message contract", () => {
  test("accepts zero scores and nullable failed tests", () => {
    for (const failedTest of [null, 0, 3]) {
      expect(submissions({
        sequence: 1, event_type: "submissions.completed",
        payload: {id, revision: 1, score: 0, state: 0, failed_test: failedTest},
      })).toBe(true);
    }
  });

  test("requires the payload belonging to the discriminator", () => {
    expect(submissions({sequence: 1, event_type: "submissions.test_started", payload: {id}})).toBe(false);
    expect(submissions({sequence: 1, event_type: "submissions.test_started", payload: {id, revision: 1, number: 1}})).toBe(true);
    expect(contests({event_type: "contest.announcement_deleted", payload: {id, contest_id: id}})).toBe(true);
    expect(contests({event_type: "contest.announcement_created", payload: {id, contest_id: id}})).toBe(false);
  });

  test("accepts frozen results without hidden fields", () => {
    expect(submissions({sequence: 1, event_type: "submissions.completed", payload: {id, revision: 1, frozen: true}})).toBe(true);
  });

  test("requires a sequence and validates authoritative scoreboard rows", () => {
    expect(submissions({event_type: "submissions.completed", payload: {id}})).toBe(false);
    const event = {event_type: "scoreboard.updated", sequence: 8, payload: {
      user_id: id, unfrozen: false, replace: false, scoreboard: {
        contest_id: id, contest_login: "round", organization_login: "org", penalty_per_attempt: 20,
        is_frozen: false, problems: [], items: [],
      },
    }};
    expect(submissions(event)).toBe(true);
    expect(submissions({...event, payload: {...event.payload, scoreboard: {}}})).toBe(false);
  });

  test("rejects unknown events, wrong field types, and missing payloads", () => {
    expect(submissions({sequence: 1, event_type: "submissions.unknown", payload: {id}})).toBe(false);
    expect(submissions({sequence: 1, event_type: "submissions.completed", payload: {id, revision: 1, score: "zero", failed_test: null}}))
      .toBe(false);
    expect(submissions({sequence: 1, event_type: "submissions.completed", payload: {score: 0}})).toBe(false);
    expect(submissions({sequence: 1, event_type: "submissions.completed"})).toBe(false);
  });
});
