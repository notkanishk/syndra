import react from "@vitejs/plugin-react";
import { resolve } from "path";
import { defineConfig } from "vitest/config";

// Per-file environment: tests opt into "jsdom" via a `// @vitest-environment jsdom`
// docblock at the top. The default is "node" so existing helper tests stay fast.
// React Testing Library tests in src/components/**/__tests__ and src/lib/queries/**
// add the docblock to get jsdom + DOM matchers.
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": resolve(__dirname, "src"),
    },
  },
  test: {
    environment: "node",
    setupFiles: ["./src/test-setup.ts"],
  },
});
