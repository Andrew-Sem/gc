export type RealGitHandoffInput = {
  readonly remoteUrl: string
  readonly workspaceRoot: string
  readonly baseBranch: string
  readonly branch: string
  readonly change: { readonly path: string; readonly content: string }
  readonly commitMessage: string
}

export type RealGitHandoffResult = {
  readonly workingDirectory: string
  readonly branch: string
  readonly headSha: string
  readonly remoteBranches: readonly string[]
  readonly commitMessage: string
}

export async function runRealGitHandoff(
  _input: RealGitHandoffInput
): Promise<RealGitHandoffResult> {
  throw new Error("production git handoff is not implemented")
}
