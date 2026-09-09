FROM node:22-bookworm-slim AS build

ENV PNPM_HOME=/pnpm
ENV PATH=$PNPM_HOME:$PATH
RUN corepack enable

WORKDIR /workspace
COPY . .
RUN pnpm install --frozen-lockfile

ARG PUBLIC_CONVEX_URL
ARG PUBLIC_WEB_URL
RUN test -n "$PUBLIC_CONVEX_URL" && test -n "$PUBLIC_WEB_URL"
RUN VITE_CONVEX_URL="$PUBLIC_CONVEX_URL" VITE_AUTH_URL="$PUBLIC_WEB_URL" \
  pnpm --filter web build

FROM nginx:1.27-alpine AS runtime

COPY docker/web-nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /workspace/apps/web/dist /usr/share/nginx/html

EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=5 \
  CMD wget --quiet --spider http://127.0.0.1:8080/health/ready || exit 1

CMD ["nginx", "-g", "daemon off;"]
