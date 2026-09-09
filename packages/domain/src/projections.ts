import type {
  CheckRun,
  DomainEvent,
  PrRef,
  PrSnapshot,
  PrStatusLine,
  RepositorySettings,
  RequiredChecksSnapshot,
  Timestamp,
} from "./contract"
import { HOUR_MS } from "./contract"
import { mergeRefusal } from "./merge-rules"

type ProjectionPr = {
  readonly pr: PrRef
  readonly createdAt: Timestamp
  readonly snapshot: PrSnapshot | null
  readonly inventories: readonly {
    readonly headSha: string
    readonly snapshot: RequiredChecksSnapshot
    readonly eventIndex: number
  }[]
  readonly observedRuns: readonly {
    readonly headSha: string
    readonly run: CheckRun
    readonly eventIndex: number
  }[]
}

function prKey(pr: PrRef): string {
  return `${pr.tenant}\u0000${pr.repo}\u0000${pr.number}`
}

function repositoryKey(tenant: string, repo: string): string {
  return `${tenant}\u0000${repo}`
}

function upsertRun(
  runs: ProjectionPr["observedRuns"],
  headSha: string,
  run: CheckRun,
  eventIndex: number
): ProjectionPr["observedRuns"] {
  const index = runs.findIndex(
    (known) => known.headSha === headSha && known.run.id === run.id
  )
  if (index < 0) return [...runs, { headSha, run, eventIndex }]
  if (run.updatedAt <= runs[index].run.updatedAt) return runs
  return runs.map((known, knownIndex) =>
    knownIndex === index ? { headSha, run, eventIndex } : known
  )
}

/**
 * Чистая свёртка журнала в модель экрана «почему PR висит». Вердикт
 * пересчитывается на текущем состоянии, а не берётся последний записанный
 * на свипе. Реализуется под таблицы why-stuck.spec.ts.
 */
export function projectOpenPrs(
  events: readonly DomainEvent[],
  now: Timestamp
): readonly PrStatusLine[] {
  const prs = new Map<string, ProjectionPr>()
  const settings = new Map<string, RepositorySettings>()

  for (const [eventIndex, event] of events.entries()) {
    switch (event.type) {
      case "PrCreated": {
        const key = prKey(event.pr)
        if (!prs.has(key)) {
          prs.set(key, {
            pr: event.pr,
            createdAt: event.at,
            snapshot: null,
            inventories: [],
            observedRuns: [],
          })
        }
        break
      }
      case "PrStateChanged": {
        const key = prKey(event.pr)
        const current = prs.get(key)
        if (
          current !== undefined &&
          (current.snapshot === null ||
            event.snapshot.updatedAt > current.snapshot.updatedAt)
        ) {
          prs.set(key, { ...current, snapshot: event.snapshot })
        }
        break
      }
      case "RequiredChecksObserved": {
        const key = prKey(event.pr)
        const current = prs.get(key)
        if (current !== undefined) {
          prs.set(key, {
            ...current,
            inventories: [
              ...current.inventories.filter(
                (inventory) => inventory.headSha !== event.headSha
              ),
              {
                headSha: event.headSha,
                snapshot: event.snapshot,
                eventIndex,
              },
            ],
          })
        }
        break
      }
      case "CheckRunStateChanged": {
        const key = prKey(event.pr)
        const current = prs.get(key)
        if (current !== undefined) {
          prs.set(key, {
            ...current,
            observedRuns: upsertRun(
              current.observedRuns,
              event.headSha,
              event.run,
              eventIndex
            ),
          })
        }
        break
      }
      case "RepositorySettingsChanged":
        settings.set(repositoryKey(event.tenant, event.repo), event.settings)
        break
    }
  }

  return [...prs.values()]
    .filter((pr) => pr.snapshot?.open === true)
    .sort((left, right) => left.pr.number - right.pr.number)
    .map((pr) => {
      const snapshot = pr.snapshot as PrSnapshot
      const currentHeadInventory = pr.inventories.find(
        (inventory) => inventory.headSha === snapshot.headSha
      )
      const inventory = currentHeadInventory ?? pr.inventories.at(-1) ?? null
      const observedRuns = pr.observedRuns
        .filter(
          (run) =>
            run.headSha === snapshot.headSha &&
            currentHeadInventory !== undefined &&
            run.eventIndex > currentHeadInventory.eventIndex
        )
        .map((run) => run.run)
      const reason = mergeRefusal(
        snapshot,
        inventory?.headSha ?? null,
        inventory?.snapshot ?? null,
        observedRuns
      )
      const closeAfterHours =
        settings.get(repositoryKey(pr.pr.tenant, pr.pr.repo))
          ?.closeAfterHours ?? 24
      const ageHours = Math.max(0, (now - pr.createdAt) / HOUR_MS)
      const hoursUntilClose = Math.max(0, closeAfterHours - ageHours)

      return {
        pr: pr.pr,
        headSha: snapshot.headSha,
        verdict:
          reason === null
            ? "will_merge"
            : inventory !== null && ageHours >= closeAfterHours
              ? "will_close"
              : "waiting",
        reason,
        ageHours,
        hoursUntilClose,
      }
    })
}
