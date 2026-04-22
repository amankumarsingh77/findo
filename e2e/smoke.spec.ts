import { test, expect } from "@playwright/test";
import { spawn } from "node:child_process";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";

function getBinaryPath(): string {
  const candidates = [
    path.join(__dirname, "..", "build", "bin", "findo"),
    path.join(__dirname, "..", "build", "bin", "findo.app", "Contents", "MacOS", "findo"),
  ];
  for (const c of candidates) {
    if (fs.existsSync(c)) return c;
  }
  throw new Error("Built binary not found. Run wails build first.");
}

let appProcess: ReturnType<typeof spawn> | null = null;
let tempDataDir: string;
let tempIndexDir: string;

test.beforeAll(async () => {
  tempDataDir = fs.mkdtempSync(path.join(os.tmpdir(), "us-e2e-data-"));
  tempIndexDir = fs.mkdtempSync(path.join(os.tmpdir(), "us-e2e-index-"));

  const files = [
    { name: "report.txt", content: "quarterly sales report results summary" },
    { name: "main.go", content: "package main\n\nfunc main() {}" },
    { name: "notes.md", content: "# Meeting notes\n\nAction items for next sprint" },
    { name: "data.json", content: '{"key": "value", "items": [1, 2, 3]}' },
    { name: "readme.txt", content: "Installation instructions for findo" },
    { name: "budget.txt", content: "annual budget planning spreadsheet data" },
    { name: "design.md", content: "# Design spec\n\nArchitecture overview" },
    { name: "config.go", content: "package config\n\ntype Config struct{}" },
    { name: "todo.txt", content: "buy groceries call dentist finish project" },
    { name: "log.txt", content: "error: connection refused at line 42" },
  ];
  for (const f of files) {
    fs.writeFileSync(path.join(tempIndexDir, f.name), f.content);
  }

  const binaryPath = getBinaryPath();
  appProcess = spawn(binaryPath, [], {
    env: { ...process.env, US_E2E_MODE: "1", US_DATA_DIR: tempDataDir },
    stdio: "ignore",
  });

  await new Promise((r) => setTimeout(r, 3000));
});

test.afterAll(async () => {
  appProcess?.kill();
  fs.rmSync(tempDataDir, { recursive: true, force: true });
  fs.rmSync(tempIndexDir, { recursive: true, force: true });
});

test("app launches and search window is accessible", async ({ page }) => {
  await page.goto("http://localhost:34115");
  await expect(page.locator('[data-testid="search-input"]')).toBeVisible({ timeout: 10_000 });
});

test("index a directory and wait for completion", async ({ page }) => {
  await page.goto("http://localhost:34115");
  await page.locator('[data-testid="add-folder-btn"]').click();
  await page.evaluate((dir) => {
    return (window as any).go.main.App.AddFolder(dir);
  }, tempIndexDir);
  await page.waitForFunction(
    async () => {
      const status = await (window as any).go.main.App.GetIndexStatus();
      return status.IsComplete === true;
    },
    { timeout: 30_000 },
  );
});

test("search returns expected file for known query", async ({ page }) => {
  await page.goto("http://localhost:34115");
  const input = page.locator('[data-testid="search-input"]');
  await input.fill("quarterly report");
  await input.press("Enter");
  await expect(page.locator('[data-testid="result-item"]').filter({ hasText: "report.txt" })).toBeVisible({
    timeout: 5_000,
  });
});

test("filter chips render for structured query", async ({ page }) => {
  await page.goto("http://localhost:34115");
  const input = page.locator('[data-testid="search-input"]');
  await input.fill("ext:go main function");
  await input.press("Enter");
  await expect(page.locator('[data-testid="filter-chip"]').filter({ hasText: "ext:go" })).toBeVisible({
    timeout: 5_000,
  });
});

test("preview panel loads for text file", async ({ page }) => {
  await page.goto("http://localhost:34115");
  const input = page.locator('[data-testid="search-input"]');
  await input.fill("installation instructions");
  await input.press("Enter");
  await page.locator('[data-testid="result-item"]').filter({ hasText: "readme.txt" }).click();
  await expect(page.locator('[data-testid="preview-panel"]')).toBeVisible({ timeout: 3_000 });
  await expect(page.locator('[data-testid="preview-panel"]')).toContainText("Installation");
});

// ---------------------------------------------------------------------------
// Failures modal smoke test
//
// This test requires a locally built Wails binary and exercises the
// indexing-failure-visibility feature end-to-end. It does NOT run by default
// in CI because it needs the built app binary and takes 30+ seconds.
//
// To run locally:
//   E2E_FAILURES_MODAL=1 npx playwright test e2e/smoke.spec.ts --grep "failures modal"
// ---------------------------------------------------------------------------
const runFailuresModal = process.env["E2E_FAILURES_MODAL"] === "1";

test.describe.configure({ mode: "serial" });

(runFailuresModal ? test.describe : test.describe.skip)(
  "failures modal — requires local binary (E2E_FAILURES_MODAL=1)",
  () => {
    let failTempDir: string;

    test.beforeAll(async () => {
      // Create a dedicated temp directory with a file that has an unsupported
      // extension (.xyz). The chunker registry has no handler for .xyz so the
      // pipeline records an ERR_UNSUPPORTED_FORMAT terminal failure.
      failTempDir = fs.mkdtempSync(path.join(os.tmpdir(), "us-e2e-fail-"));
      fs.writeFileSync(
        path.join(failTempDir, "unsupported.xyz"),
        "this file type is not in the chunker registry",
      );
    });

    test.afterAll(async () => {
      fs.rmSync(failTempDir, { recursive: true, force: true });
    });

    test("failures modal shows View button and lists the failed file", async ({ page }) => {
      await page.goto("http://localhost:34115");

      // Add the folder containing the unsupported file so the pipeline indexes it.
      await page.evaluate((dir) => {
        return (window as any).go.main.App.AddFolder(dir);
      }, failTempDir);

      // Wait until at least one failure is recorded (up to 60 s).
      await page.waitForFunction(
        async () => {
          const status = await (window as any).go.main.App.GetIndexStatus();
          return (status.FailedFiles ?? status.failedFiles ?? 0) > 0;
        },
        { timeout: 60_000 },
      );

      // The "View" button appears inside the IndexingBar whenever failedFiles > 0.
      const viewButton = page.getByRole("button", { name: "View" });
      await expect(viewButton).toBeVisible({ timeout: 5_000 });

      // Click "View" — the FailuresModal should open.
      await viewButton.click();

      // Modal is identified by its aria-label.
      const modal = page.getByRole("dialog", { name: "Indexing failures" });
      await expect(modal).toBeVisible({ timeout: 5_000 });

      // At least one group row should show the "Unsupported format" label.
      const groupLabel = modal.getByText("Unsupported format");
      await expect(groupLabel).toBeVisible({ timeout: 5_000 });

      // Expand the group by clicking its header button.
      await modal.getByRole("button", { name: /Unsupported format/ }).click();

      // After expanding, the failed filename should be visible in the detail rows.
      await expect(modal.getByText("unsupported.xyz")).toBeVisible({ timeout: 5_000 });

      // Close the modal via the footer "Close" button.
      await modal.getByRole("button", { name: "Close" }).click();
      await expect(modal).not.toBeVisible({ timeout: 3_000 });
    });
  },
);
