FROM oven/bun:1.3-alpine AS builder

WORKDIR /app

# Install dependencies
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile

# Copy source and build
COPY . .
RUN bun run build

FROM oven/bun:1.3-alpine AS runner

WORKDIR /app

ENV NODE_ENV production

# For standalone output, we still need to run the server.js file
# Bun can execute the Next.js standalone server natively
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

EXPOSE 3000
ENV PORT 3000

CMD ["bun", "server.js"]
