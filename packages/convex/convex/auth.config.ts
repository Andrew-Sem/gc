import type { AuthConfig } from "convex/server"

const convexSiteUrl =
  process.env.CONVEX_SITE_URL ??
  process.env.BETTER_AUTH_ALLOWED_ORIGIN ??
  "http://localhost:3210"

export default {
  providers: [
    {
      type: "customJwt",
      issuer: convexSiteUrl,
      applicationID: "convex",
      algorithm: "RS256",
      // The supported runtime is self-hosted Convex, where the site listener is
      // available to the verifier inside the backend container.
      jwks: "http://127.0.0.1:3211/api/auth/convex/jwks",
    },
  ],
} satisfies AuthConfig
