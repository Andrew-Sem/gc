const encoder = new TextEncoder()
const decoder = new TextDecoder()

function encode(value: Uint8Array): string {
  let binary = ""
  for (const byte of value) binary += String.fromCharCode(byte)
  return btoa(binary)
    .replaceAll("+", "-")
    .replaceAll("/", "_")
    .replace(/=+$/, "")
}

function decode(value: string): Uint8Array<ArrayBuffer> {
  const normalized = value.replaceAll("-", "+").replaceAll("_", "/")
  const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, "=")
  const binary = atob(padded)
  return Uint8Array.from(binary, (character) => character.charCodeAt(0))
}

async function encryptionKey(masterKey: string): Promise<CryptoKey> {
  const digest = await crypto.subtle.digest(
    "SHA-256",
    encoder.encode(masterKey)
  )
  return crypto.subtle.importKey("raw", digest, "AES-GCM", false, [
    "encrypt",
    "decrypt",
  ])
}

export async function encryptSecretValue(
  masterKey: string,
  plaintext: string
): Promise<string> {
  const iv = crypto.getRandomValues(new Uint8Array(12))
  const encrypted = new Uint8Array(
    await crypto.subtle.encrypt(
      { name: "AES-GCM", iv },
      await encryptionKey(masterKey),
      encoder.encode(plaintext)
    )
  )
  const value = encrypted.slice(0, -16)
  const tag = encrypted.slice(-16)
  return `w1.${encode(iv)}.${encode(value)}.${encode(tag)}`
}

export async function decryptSecretValue(
  masterKey: string,
  ciphertext: string
): Promise<string> {
  const [version, encodedIv, encodedValue, encodedTag] = ciphertext.split(".")
  if (version !== "w1" || !encodedIv || !encodedValue || !encodedTag) {
    throw new Error("invalid encrypted secret")
  }
  const value = decode(encodedValue)
  const tag = decode(encodedTag)
  const encrypted = new Uint8Array(value.length + tag.length)
  encrypted.set(value)
  encrypted.set(tag, value.length)
  const plaintext = await crypto.subtle.decrypt(
    { name: "AES-GCM", iv: decode(encodedIv) },
    await encryptionKey(masterKey),
    encrypted
  )
  return decoder.decode(plaintext)
}
