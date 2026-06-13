import { describe, it, expect, beforeEach } from "vitest";
import { getAccessToken, setAccessToken } from "./auth";

describe("auth token store", () => {
  beforeEach(() => {
    setAccessToken(null);
  });

  it("starts empty", () => {
    expect(getAccessToken()).toBeNull();
  });

  it("stores and returns a token", () => {
    setAccessToken("abc.def.ghi");
    expect(getAccessToken()).toBe("abc.def.ghi");
  });

  it("clears the token when set to null", () => {
    setAccessToken("token");
    setAccessToken(null);
    expect(getAccessToken()).toBeNull();
  });

  it("overwrites a previous token", () => {
    setAccessToken("first");
    setAccessToken("second");
    expect(getAccessToken()).toBe("second");
  });
});
