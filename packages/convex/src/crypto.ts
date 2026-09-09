import {
  createCipheriv,
  createDecipheriv,
  createHash,
  randomBytes,
  scryptSync,
} from "node:crypto"

const FORMAT_VERSION = "v1"
const WEB_FORMAT_VERSION = "w1"
const KEY_BYTES = 32
const SALT_BYTES = 16
const IV_BYTES = 12
const AUTH_TAG_BYTES = 16

function encryptionKey(masterKey: string, salt: Buffer): Buffer {
  if (!masterKey) throw new Error("master encryption key is required")
  return scryptSync(masterKey, salt, KEY_BYTES)
}

function decodePart(value: string, expectedBytes?: number): Buffer {
  if (!value || !/^[A-Za-z0-9_-]+$/.test(value)) {
    throw new Error("invalid encrypted secret")
  }
  const decoded = Buffer.from(value, "base64url")
  if (expectedBytes !== undefined && decoded.length !== expectedBytes) {
    throw new Error("invalid encrypted secret")
  }
  return decoded
}

export function encryptSecret(masterKey: string, plaintext: string): string {
  const salt = randomBytes(SALT_BYTES)
  const iv = randomBytes(IV_BYTES)
  const cipher = createCipheriv(
    "aes-256-gcm",
    encryptionKey(masterKey, salt),
    iv
  )
  cipher.setAAD(Buffer.from(FORMAT_VERSION))
  const encrypted = Buffer.concat([
    cipher.update(plaintext, "utf8"),
    cipher.final(),
  ])
  const authTag = cipher.getAuthTag()

  return [FORMAT_VERSION, salt, iv, authTag, encrypted]
    .map((part) =>
      typeof part === "string" ? part : part.toString("base64url")
    )
    .join(".")
}

export function decryptSecret(masterKey: string, ciphertext: string): string {
  if (ciphertext.startsWith(`${WEB_FORMAT_VERSION}.`)) {
    try {
      const [version, encodedIv, encodedPayload, encodedTag, extra] =
        ciphertext.split(".")
      if (
        version !== WEB_FORMAT_VERSION ||
        extra !== undefined ||
        encodedIv === undefined ||
        encodedPayload === undefined ||
        encodedTag === undefined
      ) {
        throw new Error("invalid encrypted secret")
      }
      const decipher = createDecipheriv(
        "aes-256-gcm",
        createHash("sha256").update(masterKey, "utf8").digest(),
        decodePart(encodedIv, IV_BYTES)
      )
      decipher.setAuthTag(decodePart(encodedTag, AUTH_TAG_BYTES))
      return Buffer.concat([
        decipher.update(
          encodedPayload ? decodePart(encodedPayload) : Buffer.alloc(0)
        ),
        decipher.final(),
      ]).toString("utf8")
    } catch {
      throw new Error("unable to decrypt secret")
    }
  }

  try {
    const [version, encodedSalt, encodedIv, encodedTag, encodedPayload, extra] =
      ciphertext.split(".")
    if (
      version !== FORMAT_VERSION ||
      extra !== undefined ||
      encodedSalt === undefined ||
      encodedIv === undefined ||
      encodedTag === undefined ||
      encodedPayload === undefined
    ) {
      throw new Error("invalid encrypted secret")
    }

    const salt = decodePart(encodedSalt, SALT_BYTES)
    const iv = decodePart(encodedIv, IV_BYTES)
    const authTag = decodePart(encodedTag, AUTH_TAG_BYTES)
    const payload = encodedPayload
      ? decodePart(encodedPayload)
      : Buffer.alloc(0)
    const decipher = createDecipheriv(
      "aes-256-gcm",
      encryptionKey(masterKey, salt),
      iv
    )
    decipher.setAAD(Buffer.from(FORMAT_VERSION))
    decipher.setAuthTag(authTag)
    return Buffer.concat([decipher.update(payload), decipher.final()]).toString(
      "utf8"
    )
  } catch {
    throw new Error("unable to decrypt secret")
  }
}
