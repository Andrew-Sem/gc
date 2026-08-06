import { expect, test } from "vitest"

export type Decision<Event, DomainError> =
  | { readonly ok: true; readonly events: readonly Event[] }
  | { readonly ok: false; readonly error: DomainError }

export type Decider<State, Command, Event, DomainError> = {
  readonly initialState: State
  evolve(state: State, event: Event): State
  decide(state: State, command: Command): Decision<Event, DomainError>
}

export type SpecStep<Command, Event, DomainError> = {
  readonly when: Command
} & (
  | { readonly then: readonly Event[]; readonly thenError?: never }
  | { readonly thenError: DomainError; readonly then?: never }
)

export type SpecCase<Command, Event, DomainError> = {
  readonly given: readonly Event[]
} & SpecStep<Command, Event, DomainError>

export type ExecutableTable<Command, Event, DomainError> = {
  readonly name: string
  readonly kind: "given-state" | "sequence"
  readonly given: readonly Event[]
  readonly steps: readonly SpecStep<Command, Event, DomainError>[]
}

export function accept<Event, DomainError>(
  ...events: readonly Event[]
): Decision<Event, DomainError> {
  return { ok: true, events }
}

export function reject<Event, DomainError>(
  error: DomainError
): Decision<Event, DomainError> {
  return { ok: false, error }
}

export function noop<Event, DomainError>(): Decision<Event, DomainError> {
  return { ok: true, events: [] }
}

export function assertDecision<Event, DomainError>(
  actual: Decision<Event, DomainError>,
  expected: Decision<Event, DomainError>
): void {
  expect(actual).toEqual(expected)
}

function expectedDecision<Command, Event, DomainError>(
  expectation: SpecStep<Command, Event, DomainError>
): Decision<Event, DomainError> {
  return expectation.then !== undefined
    ? { ok: true, events: expectation.then }
    : { ok: false, error: expectation.thenError as DomainError }
}

export function createExecutableSpecSuite<
  State,
  Command,
  Event,
  DomainError,
>(options: {
  readonly decider: Decider<State, Command, Event, DomainError>
  readonly enabled?: boolean
  readonly prepareStep?: (
    step: SpecStep<Command, Event, DomainError>
  ) => SpecStep<Command, Event, DomainError>
}) {
  const catalog: ExecutableTable<Command, Event, DomainError>[] = []
  const enabled = options.enabled ?? true
  const prepareStep =
    options.prepareStep ??
    ((step: SpecStep<Command, Event, DomainError>) => step)

  function runCase(
    name: string,
    given: readonly Event[],
    step: SpecStep<Command, Event, DomainError>
  ): void {
    if (!enabled) return

    test(name, () => {
      const state = given.reduce(
        options.decider.evolve,
        options.decider.initialState
      )
      const actual = options.decider.decide(state, step.when)
      assertDecision(actual, expectedDecision(step))
    })
  }

  function spec(
    name: string,
    scenario: SpecCase<Command, Event, DomainError>
  ): void {
    const step = prepareStep(scenario)
    catalog.push({
      name,
      kind: "given-state",
      given: scenario.given,
      steps: [step],
    })
    runCase(name, scenario.given, step)
  }

  function examples(
    name: string,
    scenarios: readonly SpecCase<Command, Event, DomainError>[]
  ): void {
    for (const [index, scenario] of scenarios.entries()) {
      const step = prepareStep(scenario)
      const tableName = `${name} [table ${index + 1}/${scenarios.length}]`
      catalog.push({
        name: tableName,
        kind: "given-state",
        given: scenario.given,
        steps: [step],
      })
      runCase(tableName, scenario.given, step)
    }
  }

  function sequence(
    name: string,
    given: readonly Event[],
    steps: readonly SpecStep<Command, Event, DomainError>[]
  ): void {
    const preparedSteps = steps.map(prepareStep)
    catalog.push({ name, kind: "sequence", given, steps: preparedSteps })
    if (!enabled) return

    test(name, () => {
      let state = given.reduce(
        options.decider.evolve,
        options.decider.initialState
      )
      for (const step of preparedSteps) {
        const actual = options.decider.decide(state, step.when)
        assertDecision(actual, expectedDecision(step))
        if (actual.ok) {
          state = actual.events.reduce(options.decider.evolve, state)
        }
      }
    })
  }

  return { catalog, examples, sequence, spec }
}

export function createSpec<State, Command, Event, DomainError>(
  decider: Decider<State, Command, Event, DomainError>
) {
  return createExecutableSpecSuite({ decider }).spec
}
