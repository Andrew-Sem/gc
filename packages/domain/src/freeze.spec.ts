import { createHash } from "node:crypto"
import { readFileSync } from "node:fs"
import { dirname, join } from "node:path"
import { fileURLToPath } from "node:url"
import { expect, test } from "vitest"
import manifest from "../freeze-manifest.json" with { type: "json" }

const packageRoot = join(dirname(fileURLToPath(import.meta.url)), "..")

function sha256(relativePath: string): string {
  return createHash("sha256")
    .update(readFileSync(join(packageRoot, relativePath)))
    .digest("hex")
}

const frozen: readonly (readonly [string, string, string])[] = [
  ...Object.entries(manifest.shared.files).map(
    ([path, hash]) => ["shared", path, hash] as const
  ),
  ...Object.entries(manifest.slices).flatMap(([slice, entry]) =>
    Object.entries(entry.files).map(
      ([path, hash]) => [slice, path, hash] as const
    )
  ),
]

test.each(frozen)("%s: %s не менялся после заморозки", (_slice, path, hash) => {
  expect(sha256(path)).toBe(hash)
})
