import { defineConfig } from "@playwright/test";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(root, "..");

export default defineConfig({
  testDir: "./tests",
  workers: 1,
  timeout: 120_000,
  use: {
    baseURL: process.env.SUBTILL_E2E_BASE_URL ?? "http://127.0.0.1:9245",
    trace: "on-first-retry",
  },
  webServer: {
    command: `mkdir -p e2e/.tmp-config && ./bin/subtill-server --config-path e2e/.tmp-config`,
    cwd: repoRoot,
    url: process.env.SUBTILL_E2E_BASE_URL ?? "http://127.0.0.1:9245",
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
