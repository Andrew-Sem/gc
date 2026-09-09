ARG CONVEX_BACKEND_IMAGE=ghcr.io/get-convex/convex-backend@sha256:240a2e9115dab3c13145b96532d1cc8bc63c5e2d112d95f80e592a75242bfa75

FROM node:22-bookworm-slim AS build

ENV PNPM_HOME=/pnpm
ENV PATH=$PNPM_HOME:$PATH
RUN corepack enable

WORKDIR /workspace
COPY . .
RUN pnpm install --frozen-lockfile
RUN pnpm --filter @workspace/worker exec esbuild ../../packages/convex/src/runtime-env.ts \
  --bundle --platform=node --format=esm --outfile=/runtime-env.mjs

FROM ${CONVEX_BACKEND_IMAGE} AS runtime

COPY --from=build /workspace /app
RUN rm /app/packages/convex/convex/test-modules.ts
COPY --from=build /runtime-env.mjs /opt/garbage-collector/runtime-env.mjs
COPY --chmod=755 docker/convex-entrypoint.sh /usr/local/bin/start-garbage-collector-convex

EXPOSE 3210 3211
HEALTHCHECK --interval=5s --timeout=3s --start-period=20s --retries=20 \
  CMD curl --fail --silent http://localhost:3211/health/ready || exit 1

ENTRYPOINT ["/usr/local/bin/start-garbage-collector-convex"]
