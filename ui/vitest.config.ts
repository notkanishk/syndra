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
    // jsdom defaults to `about:blank`, which is an opaque origin, and an
    // opaque origin has no usable storage — `localStorage.setItem` is not a
    // function there. Everything in this product that remembers a choice goes
    // through localStorage: the theme, the Basic/Advanced view, the drift
    // chime. Without an origin none of them can be tested at all, which is
    // why none of them were.
    environmentOptions: { jsdom: { url: "http://localhost" } },
    setupFiles: ["./src/test-setup.ts"],
  },
});
