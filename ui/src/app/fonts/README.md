# Fonts

The three faces the app sets in `../layout.tsx`, self-hosted so a build never
depends on reaching Google Fonts (a deploy failed on 5 Nov 2026 when it could not).
Each is Google's latin subset of the variable font, unchanged:

- `bricolage-latin.woff2`: Bricolage Grotesque, wght 200-800 and opsz 12-96
- `figtree-latin.woff2`: Figtree, wght 400-700 (variable)
- `jetbrains-latin.woff2`: JetBrains Mono, wght 400-500 (variable)

All three are under the SIL Open Font License 1.1 (https://openfontlicense.org).
To refresh one, fetch `https://fonts.googleapis.com/css2?family=<family>&display=swap`
with a modern browser user agent and take the file from its `/* latin */` block.
