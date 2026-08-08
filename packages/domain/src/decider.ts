import type { DomainCommand, DomainError, DomainEvent } from "./contract"
import { accept, type Decider, type Decision } from "./framework/runner"

export type Settings = {
  readonly openPrLimit: number
}

export type GcState = {
  readonly settings: Settings
  readonly openPrs: readonly number[]
}

export const initialState: GcState = {
  settings: { openPrLimit: 12 },
  openPrs: [],
}

export function evolve(state: GcState, event: DomainEvent): GcState {
  switch (event.type) {
    case "PrCreated":
      return state.openPrs.includes(event.pr.number)
        ? state
        : { ...state, openPrs: [...state.openPrs, event.pr.number] }
    case "PrStateChanged":
      return event.snapshot.open
        ? state
        : {
            ...state,
            openPrs: state.openPrs.filter((n) => n !== event.pr.number),
          }
    default:
      return state
  }
}

export function decide(
  state: GcState,
  command: DomainCommand
): Decision<DomainEvent, DomainError> {
  if (command.type !== "TickProduction") return accept()

  const open = state.openPrs.length
  const limit = state.settings.openPrLimit

  if (open >= limit) {
    return accept({
      type: "AdmissionDeclined",
      tenant: command.tenant,
      repo: command.repo,
      refusal: { kind: "backlog_full", open, limit },
      at: command.now,
    })
  }

  return accept({
    type: "AdmissionApproved",
    tenant: command.tenant,
    repo: command.repo,
    tenantSettings: command.tenantSettings,
    at: command.now,
  })
}

export const decider: Decider<
  GcState,
  DomainCommand,
  DomainEvent,
  DomainError
> = {
  initialState,
  evolve,
  decide,
}
