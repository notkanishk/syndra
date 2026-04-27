import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

// @testing-library/react auto-cleans only with vitest globals:true. We import
// explicitly, so wire cleanup() into afterEach so jsdom DOM doesn't leak
// across tests (would otherwise produce "multiple role=dialog" errors).
afterEach(() => {
  cleanup();
});
