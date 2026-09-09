import {
  type DomainCommand,
  type DomainEvent,
  decide,
  type Effect,
  effectsOf,
  evolve,
  initialState,
} from "@workspace/domain"
import type { GenericMutationCtx, GenericQueryCtx } from "convex/server"
import type { DataModel } from "../src/data-model"

type DbReader = GenericQueryCtx<DataModel>["db"]
type DbWriter = GenericMutationCtx<DataModel>["db"]
type DispatchOptions = {
  readonly runAgentSnapshot?: { readonly openaiBaseUrl?: string }
  readonly handleEffect?: (effect: Effect) => Promise<boolean>
}

export async function journalEvents(db: DbReader): Promise<DomainEvent[]> {
  const rows = await db.query("journal").collect()
  return rows.map((row) => row.event as DomainEvent)
}

function effectTenant(effect: Effect): string {
  return effect.type === "RunAgent" ? effect.tenant : effect.pr.tenant
}

function effectRepo(effect: Effect): string {
  return effect.type === "RunAgent" ? effect.repo : effect.pr.repo
}

export async function enqueueEffect(
  db: DbWriter,
  effect: unknown,
  options?: DispatchOptions
): Promise<void> {
  const typed = effect as Effect
  await db.insert("tasks", {
    status: "pending",
    executionCount: 0,
    effect,
    tenant: effectTenant(typed),
    repo: effectRepo(typed),
    reportSubmissions: 0,
    ...(typed.type === "RunAgent" && options?.runAgentSnapshot
      ? { runAgentSnapshot: options.runAgentSnapshot }
      : {}),
  })
}

export async function dispatchCommand(
  db: DbWriter,
  command: DomainCommand,
  options?: DispatchOptions
): Promise<readonly DomainEvent[]> {
  const events = await journalEvents(db)
  const state = events.reduce(evolve, initialState)
  const decision = decide(state, command)
  if (!decision.ok) return []

  for (const event of decision.events) {
    await db.insert("journal", { event })
  }
  for (const effect of effectsOf(decision.events)) {
    if (!(await options?.handleEffect?.(effect))) {
      await enqueueEffect(db, effect, options)
    }
  }
  return decision.events
}

export function latestByType<T extends DomainEvent["type"]>(
  events: readonly DomainEvent[],
  type: T,
  predicate: (event: Extract<DomainEvent, { type: T }>) => boolean
): Extract<DomainEvent, { type: T }> | null {
  for (let index = events.length - 1; index >= 0; index -= 1) {
    const event = events[index]
    if (event.type === type && predicate(event as never)) {
      return event as Extract<DomainEvent, { type: T }>
    }
  }
  return null
}

export function connectedRepositories(
  events: readonly DomainEvent[]
): readonly { readonly tenant: string; readonly repo: string }[] {
  const connected = new Map<string, { tenant: string; repo: string }>()
  for (const event of events) {
    if (event.type !== "RepositoryConnected") continue
    connected.set(`${event.tenant}\u0000${event.repo}`, {
      tenant: event.tenant,
      repo: event.repo,
    })
  }
  return [...connected.values()]
}

export async function eligibleConnectedRepositories(
  db: DbReader,
  events: readonly DomainEvent[]
): Promise<readonly { readonly tenant: string; readonly repo: string }[]> {
  const installations = await db.query("installations").collect()
  const eligible = new Map(
    installations
      .filter(
        (installation) => !installation.suspended && !installation.deleted
      )
      .map((installation) => [
        installation.tenant,
        new Set(installation.availableRepositories),
      ])
  )
  return connectedRepositories(events).filter(({ tenant, repo }) =>
    eligible.get(tenant)?.has(repo)
  )
}

export function reportId(prefix: string): string {
  return `${prefix}:${crypto.randomUUID()}`
}
