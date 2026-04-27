import test from "node:test";
import assert from "node:assert/strict";
import { cliBinaryName, cliSkillName, fetchRegistry, lookupByName, parseRegistry } from "../src/registry.js";

test("parseRegistry validates and returns registry entries", () => {
  const registry = parseRegistry({
    schema_version: 1,
    entries: [
      {
        name: "espn",
        category: "sports",
        api: "ESPN",
        description: "Sports scores",
        path: "library/sports/espn",
      },
    ],
  });

  assert.equal(registry.entries.length, 1);
  assert.equal(registry.entries[0]?.name, "espn");
});

test("lookupByName matches normalized CLI and API names", () => {
  const registry = parseRegistry({
    schema_version: 1,
    entries: [
      {
        name: "yahoo-finance-pp-cli",
        category: "finance",
        api: "Yahoo Finance",
        description: "Market data",
        path: "library/finance/yahoo-finance",
      },
    ],
  });

  assert.equal(lookupByName(registry, "yahoo-finance")?.path, "library/finance/yahoo-finance");
  assert.equal(lookupByName(registry, "pp-yahoo-finance")?.path, "library/finance/yahoo-finance");
  assert.equal(lookupByName(registry, "Yahoo Finance")?.path, "library/finance/yahoo-finance");
  assert.equal(lookupByName(registry, "missing"), null);
});

test("cliSkillName preserves pp- naming convention", () => {
  const registry = parseRegistry({
    schema_version: 1,
    entries: [
      {
        name: "dominos-pp-cli",
        category: "commerce",
        api: "Dominos",
        description: "Pizza ordering",
        path: "library/commerce/dominos",
      },
    ],
  });

  assert.equal(cliSkillName(registry.entries[0]!), "pp-dominos");
  assert.equal(cliBinaryName(registry.entries[0]!), "dominos-pp-cli");
});

test("parseRegistry rejects unsupported schema versions", () => {
  assert.throws(() => parseRegistry({ schema_version: 2, entries: [] }), /unsupported registry/);
});

test("fetchRegistry sends GitHub token when available", async () => {
  const previous = process.env.GITHUB_TOKEN;
  process.env.GITHUB_TOKEN = "test-token";
  let authHeader: string | null = null;
  try {
    await fetchRegistry("https://example.test/registry.json", async (_url, init) => {
      authHeader = new Headers(init?.headers).get("authorization");
      return new Response(
        JSON.stringify({
          schema_version: 1,
          entries: [],
        }),
        { status: 200 },
      );
    });
  } finally {
    if (previous === undefined) {
      delete process.env.GITHUB_TOKEN;
    } else {
      process.env.GITHUB_TOKEN = previous;
    }
  }

  assert.equal(authHeader, "Bearer test-token");
});
