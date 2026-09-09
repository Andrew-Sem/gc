FROM node:22-bookworm-slim AS build

ENV PNPM_HOME=/pnpm
ENV PATH=$PNPM_HOME:$PATH
RUN corepack enable

WORKDIR /workspace
COPY . .
RUN pnpm install --frozen-lockfile
RUN pnpm --filter @workspace/worker build
RUN pnpm --filter @workspace/worker --prod deploy --legacy /worker-runtime

FROM node:22-bookworm-slim AS runtime

RUN apt-get update \
  && apt-get install --yes --no-install-recommends ca-certificates curl git libgcc-s1 libstdc++6 \
  && rm -rf /var/lib/apt/lists/* \
  && groupadd --gid 10001 worker \
  && useradd --uid 10001 --gid worker --create-home --shell /usr/sbin/nologin worker \
  && mkdir -p /app /data/codex-home /workspaces \
  && chown -R worker:worker /app /data/codex-home /workspaces

COPY --from=build --chown=worker:worker /workspace/apps/worker/dist/main.js /app/main.js
COPY --from=build --chown=worker:worker /worker-runtime/node_modules /app/node_modules
COPY --chown=worker:worker docker/worker-entrypoint.mjs /app/worker-entrypoint.mjs
RUN set -- /app/node_modules/.pnpm/@openai+codex@*/node_modules/@openai/codex/bin/codex.js \
  && test -x "$1" \
  && ln -s "$1" /usr/local/bin/codex

ENV NODE_ENV=production
WORKDIR /app
EXPOSE 8081
USER worker

HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=5 \
  CMD curl --fail --silent http://localhost:8081/health/ready || exit 1

CMD ["node", "/app/worker-entrypoint.mjs"]
