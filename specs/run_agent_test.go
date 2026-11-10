package specs

import (
	"encoding/json"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/Andrew-Sem/gc/specs/contract"
	"github.com/Andrew-Sem/gc/specs/extensions"
)

var answerFields = map[string]string{
	"slug":                "dead-code",
	"title":               "Remove dead code",
	"description":         "The helper has no callers, so it is removed.",
	"finding":             "The helper has no callers.",
	"evidence":            "A search finds no other use.",
	"selected_candidate":  "Remove the helper.",
	"rejected_candidates": "Rename it: nothing calls it.",
	"change":              "Delete the helper.",
	"validation":          "The tests pass.",
	"risk":                "Low.",
	"rollback":            "Revert the commit.",
	"follow_up":           "None",
}

var answer = answerWith(nil)

const report = `## Finding

The helper has no callers.

## Evidence

A search finds no other use.

## Selected Candidate

Remove the helper.

## Rejected Candidates

Rename it: nothing calls it.

## Change

Delete the helper.

## Validation

The tests pass.

## Risk

Low.

## Rollback

Revert the commit.

## Follow-up

None
`

const description = "The helper has no callers, so it is removed.\n\nReport: "

func answerWith(changes map[string]string) string {
	fields := maps.Clone(answerFields)
	maps.Copy(fields, changes)
	encoded, err := json.Marshal(fields)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func answerWithout(field string) string {
	fields := maps.Clone(answerFields)
	delete(fields, field)
	encoded, err := json.Marshal(fields)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func TestRunAgent(t *testing.T) {
	f := extensions.NewFeature(t, "run-agent.md")

	f.RunAgent("hand over a change", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.1.1").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the change is on a new branch and main is untouched").
			ExpectChange("main", "Remove dead code", "cleaned.txt", "clean")
	})

	f.RunAgent("write the report", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.5.1", "AGENT-3.5.2").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the commit adds one report named after the slug, and main is untouched").
			ExpectReport("dead-code", report)
	})

	f.RunAgent("the report in the description leaves no file", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.5.10", "AGENT-3.5.11").
			GivenRepositoryWithCommits(1).
			GivenTaskReportInDescription().
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the branch has only the agent's change, and the description holds the report").
			ExpectChange("main", "Remove dead code", "cleaned.txt", "clean").
			ExpectBranchFiles("cleaned.txt").
			ExpectDescription("The helper has no callers, so it is removed.\n\n" + report)
	})

	f.RunAgent("the PR description points to the report", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.1.2", "AGENT-3.5.3").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the description is the agent's short text and ends with the report's path").
			ExpectDescriptionEndingWithReportPath(description)
	})

	for _, tt := range []struct{ name, slug string }{
		{"fail when the slug leaves the folder", "../../evil"},
		{"fail when the slug has capital letters", "Dead-Code"},
		{"fail when the slug has other characters", "dead.code"},
		{"fail when the slug is too long", "a123456789b123456789c123456789d123456789e123456789f123456789g"},
	} {
		f.RunAgent(tt.name, func(s *extensions.AgentRun) {
			s.Covers("AGENT-3.5.4").
				GivenRepositoryWithCommits(1).
				GivenAgentChanges("cleaned.txt", "clean").
				GivenAgentAnswers(answerWith(map[string]string{"slug": tt.slug})).
				WhenAgentRuns().
				Then("the run fails and nothing is pushed").
				ExpectFailure().
				ExpectBranches(0, "gc/")
		})
	}

	f.RunAgent("fail when an answer field is missing", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.5.5").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentAnswers(answerWithout("rollback")).
			WhenAgentRuns().
			Then("the run fails and nothing is pushed").
			ExpectFailure().
			ExpectBranches(0, "gc/")
	})

	f.RunAgent("fail when an answer field is empty", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.5.5").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentAnswers(answerWith(map[string]string{"evidence": " "})).
			WhenAgentRuns().
			Then("the run fails and nothing is pushed").
			ExpectFailure().
			ExpectBranches(0, "gc/")
	})

	f.RunAgent("clone and push with an installation token", func(s *extensions.AgentRun) {
		s.Covers("AGENT-1.1.3", "AGENT-3.1.4").
			GivenInstallationToken("another-github-token").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the repository saw that token on the clone and on the push").
			ExpectGitAuthenticated("another-github-token")
	})

	f.RunAgent("each token is limited to the run's repository", func(s *extensions.AgentRun) {
		s.Covers("AGENT-1.1.4").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the clone and the push used tokens for that repository alone").
			ExpectBranches(1, "gc/").
			ExpectTokensLimitedToTheRepository()
	})

	f.RunAgent("the commit comes from the app's bot user", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.1.6").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("GitHub shows the app as the author").
			ExpectCommittedBy("gc-spec[bot] <4711+gc-spec[bot]@users.noreply.github.com>")
	})

	f.RunAgent("no commit when GitHub cannot name the app", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.1.6").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentAnswers(answer).
			GivenGitHubCannotNameTheApp().
			WhenAgentRuns().
			Then("the run fails and nothing is pushed").
			ExpectFailure().
			ExpectBranches(0, "gc/")
	})

	f.RunAgent("push after the clone's token has expired", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.1.5").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentAnswers(answer).
			GivenTokensExpireWhileTheAgentWorks().
			WhenAgentRuns().
			Then("gc pushes with a new token").
			ExpectChange("main", "Remove dead code", "cleaned.txt", "clean")
	})

	f.RunAgent("each run gets its own branch", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.1.3").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentAnswers(answer).
			WhenAgentRunsTwice().
			Then("two different gc/ branches are pushed").
			ExpectBranches(2, "gc/")
	})

	f.RunAgent("only the latest commit is cloned", func(s *extensions.AgentRun) {
		s.Covers("AGENT-1.1.1").
			GivenRepositoryWithCommits(3).
			WhenAgentRuns().
			Then("the agent sees one commit of the default branch").
			ExpectAgentSaw(extensions.AgentSaw{Commits: 1, Branch: "main"})
	})

	f.RunAgent("no agent when the clone fails", func(s *extensions.AgentRun) {
		s.Covers("AGENT-1.1.2").
			GivenNoRepository().
			WhenAgentRuns().
			Then("the run fails before the agent starts").
			ExpectFailure().
			ExpectAgentNotStarted()
	})

	f.RunAgent("remove the working copy after a change", func(s *extensions.AgentRun) {
		s.Covers("AGENT-1.2.1").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the folder is gone").
			ExpectAgentLeftNothing()
	})

	f.RunAgent("remove the working copy after a failure", func(s *extensions.AgentRun) {
		s.Covers("AGENT-1.2.1").
			GivenRepositoryWithCommits(1).
			GivenAgentExitsWithError().
			WhenAgentRuns().
			Then("the folder is gone").
			ExpectFailure().
			ExpectAgentLeftNothing()
	})

	f.RunAgent("remove a read-only folder the agent left in the working copy", func(s *extensions.AgentRun) {
		s.Covers("AGENT-1.2.1", "AGENT-1.2.2").
			GivenRepositoryWithCommits(1).
			GivenAgentRuns(`mkdir -p stuck/inner; echo x > stuck/inner/file; chmod 555 stuck/inner stuck`).
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the change is pushed and the folder is gone").
			ExpectBranches(1, "gc/").
			ExpectAgentLeftNothing()
	})

	f.RunAgent("remove closed folders the agent left in its home and temporary folder", func(s *extensions.AgentRun) {
		s.Covers("AGENT-1.2.2", "AGENT-2.1.5", "AGENT-2.2.5").
			GivenRepositoryWithCommits(1).
			GivenAgentRuns(`for d in "$CODEX_HOME" "$TMPDIR"; do mkdir -p "$d/stuck"; echo x > "$d/stuck/file"; chmod 000 "$d/stuck"; done`).
			WhenAgentRuns().
			Then("both are gone").
			ExpectNoChange().
			ExpectAgentLeftNothing()
	})

	f.RunAgent("start the agent in the sandbox", func(s *extensions.AgentRun) {
		s.Covers("AGENT-2.1.1", "AGENT-2.1.2").
			GivenRepositoryWithCommits(1).
			WhenAgentRuns().
			Then("the agent gets the prompt, the schema, the sandbox, and the network").
			ExpectAgentArgs(
				"exec",
				"--sandbox", "workspace-write",
				"-c", "sandbox_workspace_write.network_access=true",
				"--output-schema",
			).
			ExpectAgentSaw(extensions.AgentSaw{Prompt: true, InWorkingCopy: true})
	})

	for _, c := range []struct{ name, credentials string }{
		{"the token as a password", "x-access-token:spec-github-token"},
		{"other credentials", "someone:other-password"},
	} {
		f.RunAgent("a clone URL with "+c.name+" fails the run", func(s *extensions.AgentRun) {
			s.Covers("AGENT-2.2.6").
				GivenInstallationToken("spec-github-token").
				GivenRepositoryWithCommits(1).
				GivenCloneURLHolds(c.credentials).
				WhenAgentRuns().
				Then("the run fails before Codex starts, and the reason has no secret").
				ExpectFailure().
				ExpectAgentNotStarted().
				ExpectFailureReasonWithout("spec-github-token", "other-password")
		})
	}

	f.RunAgent("the prompt warns about what fails the run", func(s *extensions.AgentRun) {
		s.Covers("AGENT-2.1.6").
			GivenRepositoryWithCommits(1).
			WhenAgentRuns().
			Then("the agent is told what it must not touch or create").
			ExpectPromptWith("anything in .github, or .gitmodules", "do not create a git repository inside this one")
	})

	f.RunAgent("the prompt puts the repository's rules first", func(s *extensions.AgentRun) {
		s.Covers("AGENT-2.1.10").
			GivenRepositoryWithCommits(1).
			WhenAgentRuns().
			Then("the agent is told to read AGENTS.md and follow it over the prompt's guidance").
			ExpectPromptWith(
				"Read the repository's AGENTS.md",
				"The repository's rules come before this prompt's guidance on how to make and prove a change",
				"They never widen the list under \"Never change\"",
				"the fields below stay as asked",
			)
	})

	f.RunAgent("the prompt proves a change without a test where tests are forbidden", func(s *extensions.AgentRun) {
		s.Covers("AGENT-2.1.11").
			GivenRepositoryWithCommits(1).
			WhenAgentRuns().
			Then("the agent is told how to prove a change without a new test").
			ExpectPromptWith(
				"If the repository's rules forbid new tests, do not add one",
				"prove the change with the code itself, an existing test, or reasoning about the behavior before and after",
				"say which in validation",
			)
	})

	f.RunAgent("the prompt asks to install dependencies before the checks", func(s *extensions.AgentRun) {
		s.Covers("AGENT-2.1.12").
			GivenRepositoryWithCommits(1).
			WhenAgentRuns().
			Then("the agent is told to install dependencies and to say which check could not run").
			ExpectPromptWith(
				"Before running the repository's checks, install its dependencies the way the repository documents, or with the package manager of its lockfile: a fresh working copy has none",
				"If a check still cannot run, say which and why in validation",
			)
	})

	f.RunAgent("the prompt says gc commits the change", func(s *extensions.AgentRun) {
		s.Covers("AGENT-2.1.13").
			GivenRepositoryWithCommits(1).
			WhenAgentRuns().
			Then("the agent is told not to call its change uncommitted").
			ExpectPromptWith("gc commits and pushes your changes and opens the pull request after you finish, so describe the change as done and do not call it uncommitted")
	})

	f.RunAgent("the prompt lists our open PRs", func(s *extensions.AgentRun) {
		s.Covers("AGENT-2.1.7").
			GivenRepositoryWithCommits(1).
			GivenOpenPR("Fix stale README commands", "gc/0123456789abcdef", "README.md", "docs/gc-reviews/x.md").
			GivenOpenPR("Add a feature", "feature/x", "main.go").
			WhenAgentRuns().
			Then("the agent knows what is already proposed, and only ours").
			ExpectPromptWith(`"Fix stale README commands", changing README.md, docs/gc-reviews/x.md`, "Do not repeat their fix").
			ExpectPromptWithout("Add a feature", "main.go")
	})

	f.RunAgent("with none of our PRs open the prompt has no list", func(s *extensions.AgentRun) {
		s.Covers("AGENT-2.1.7").
			GivenRepositoryWithCommits(1).
			GivenOpenPR("Add a feature", "feature/x", "main.go").
			WhenAgentRuns().
			Then("the prompt is the plain prompt").
			ExpectAgentSaw(extensions.AgentSaw{Prompt: true})
	})

	f.RunAgent("many open PRs, each slow to list, still reach the prompt", func(s *extensions.AgentRun) {
		s.Covers("AGENT-2.1.9").
			GivenRepositoryWithCommits(1).
			GivenOpenPR("Fix one", "gc/01", "one.md").
			GivenOpenPR("Fix two", "gc/02", "two.md").
			GivenOpenPR("Fix three", "gc/03", "three.md").
			GivenGitHubListsPRsAfter(2*time.Second).
			WhenAgentRuns().
			Then("eight seconds of listing in all, under five each, and every PR is in the prompt").
			ExpectPromptWith(`"Fix one", changing one.md`, `"Fix two", changing two.md`, `"Fix three", changing three.md`)
	})

	f.RunAgent("no run when GitHub cannot list the open PRs", func(s *extensions.AgentRun) {
		s.Covers("AGENT-2.1.8").
			GivenRepositoryWithCommits(1).
			GivenGitHubCannotListPRs().
			WhenAgentRuns().
			Then("the run fails before Codex starts").
			ExpectFailure().
			ExpectAgentNotStarted()
	})

	f.RunAgent("keep secrets away from the agent", func(s *extensions.AgentRun) {
		s.Covers("AGENT-2.2.1", "AGENT-2.2.3").
			GivenInstallationToken("spec-github-token").
			GivenEnv("GC_WEBHOOK_SECRET", "spec-webhook-secret").
			GivenEnv("OPENAI_API_KEY", "spec-openai-key").
			GivenRepositoryWithCommits(1).
			WhenAgentRuns().
			Then("only the OpenAI key reaches the agent").
			ExpectAgentEnvWithout("GITHUB_APP_PRIVATE_KEY_FILE", "GC_WEBHOOK_SECRET").
			ExpectAgentEnv("CODEX_API_KEY", "spec-openai-key")
	})

	f.RunAgent("use a local proxy instead of OpenAI", func(s *extensions.AgentRun) {
		s.Covers("AGENT-2.1.3").
			GivenTaskBaseURL("http://127.0.0.1:8317/v1").
			GivenRepositoryWithCommits(1).
			WhenAgentRuns().
			Then("the agent is told the address").
			ExpectAgentArgs("-c", `openai_base_url="http://127.0.0.1:8317/v1"`)
	})

	f.RunAgent("use OpenAI without a base URL", func(s *extensions.AgentRun) {
		s.Covers("AGENT-2.1.3").
			GivenEnv("OPENAI_BASE_URL", "http://127.0.0.1:8317/v1").
			GivenRepositoryWithCommits(1).
			WhenAgentRuns().
			Then("the agent is told no address, whatever gc's environment says").
			ExpectAgentArgsWithout("openai_base_url")
	})

	f.RunAgent("the agent does not wait for input", func(s *extensions.AgentRun) {
		s.Covers("AGENT-2.1.4").
			GivenRepositoryWithCommits(1).
			GivenAgentReadsInput().
			WhenAgentRuns().
			Then("the run finishes").
			ExpectNoChange()
	})

	f.RunAgent("the agent gets its own empty home", func(s *extensions.AgentRun) {
		s.Covers("AGENT-2.1.5").
			GivenEnv("CODEX_HOME", "/operator/codex").
			GivenRepositoryWithCommits(1).
			WhenAgentRuns().
			Then("the operator's home is not used, and the run's home is gone afterwards").
			ExpectAgentSaw(extensions.AgentSaw{EmptyHome: true}).
			ExpectAgentEnvWithout("/operator/codex").
			ExpectAgentLeftNothing()
	})

	f.RunAgent("shared temporary folders are closed to the agent", func(s *extensions.AgentRun) {
		s.Covers("AGENT-2.2.4").
			GivenRepositoryWithCommits(1).
			WhenAgentRuns().
			Then("the sandbox leaves out TMPDIR and /tmp").
			ExpectAgentArgs(
				"-c", "sandbox_workspace_write.exclude_tmpdir_env_var=true",
				"-c", "sandbox_workspace_write.exclude_slash_tmp=true",
			)
	})

	f.RunAgent("the agent gets its own temporary folder", func(s *extensions.AgentRun) {
		s.Covers("AGENT-2.2.5").
			GivenEnv("TMPDIR", "/operator/tmp").
			GivenRepositoryWithCommits(1).
			WhenAgentRuns().
			Then("the operator's folder is not used, and the run's folder is gone afterwards").
			ExpectAgentSaw(extensions.AgentSaw{OwnTemporaryFolder: true}).
			ExpectAgentEnvWithout("/operator/tmp").
			ExpectAgentLeftNothing()
	})

	f.RunAgent("no token in the working copy", func(s *extensions.AgentRun) {
		s.Covers("AGENT-2.2.2").
			GivenInstallationToken("spec-github-token").
			GivenRepositoryWithCommits(1).
			WhenAgentRuns().
			Then("the token is nowhere in the folder the agent sees").
			ExpectWorkingCopyWithout("spec-github-token")
	})

	f.RunAgent("no change when the agent changed nothing", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.2.1", "AGENT-3.5.6").
			GivenRepositoryWithCommits(1).
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the run returns no change").
			ExpectNoChange().
			ExpectBranches(0, "gc/")
	})

	f.RunAgent("explain why there is no change", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.2.2").
			GivenRepositoryWithCommits(1).
			GivenAgentAnswers(answerWith(map[string]string{"finding": "The helper has no callers.\n"})).
			WhenAgentRuns().
			Then("the explanation holds the finding and the candidates, as in the report").
			ExpectNoChange().
			ExpectExplanation("## Finding\n\nThe helper has no callers.\n\n" +
				"## Selected Candidate\n\nRemove the helper.\n\n" +
				"## Rejected Candidates\n\nRename it: nothing calls it.")
	})

	f.RunAgent("an empty field is left out of the explanation", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.2.3").
			GivenRepositoryWithCommits(1).
			GivenAgentAnswers(answerWith(map[string]string{"finding": " ", "rejected_candidates": ""})).
			WhenAgentRuns().
			Then("only the selected candidate is left").
			ExpectNoChange().
			ExpectExplanation("## Selected Candidate\n\nRemove the helper.")
	})

	f.RunAgent("a missing field is left out of the explanation", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.2.3").
			GivenRepositoryWithCommits(1).
			GivenAgentAnswers(answerWithout("selected_candidate")).
			WhenAgentRuns().
			Then("the finding and the rejected candidates are left").
			ExpectNoChange().
			ExpectExplanation("## Finding\n\nThe helper has no callers.\n\n" +
				"## Rejected Candidates\n\nRename it: nothing calls it.")
	})

	f.RunAgent("no explanation when the answer is not JSON", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.2.3").
			GivenRepositoryWithCommits(1).
			GivenAgentAnswers("Nothing to do!").
			WhenAgentRuns().
			Then("the run returns no change with an empty explanation").
			ExpectNoChange().
			ExpectExplanation("")
	})

	f.RunAgent("no explanation when every field is empty", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.2.3").
			GivenRepositoryWithCommits(1).
			GivenAgentAnswers(answerWith(map[string]string{"finding": "", "selected_candidate": "", "rejected_candidates": ""})).
			WhenAgentRuns().
			Then("the run returns no change with an empty explanation").
			ExpectNoChange().
			ExpectExplanation("")
	})

	f.RunAgent("a long explanation is cut on a whole character", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.2.4").
			GivenRepositoryWithCommits(1).
			GivenAgentAnswers(answerWith(map[string]string{
				"finding":             "x" + strings.Repeat("ж", 3000),
				"selected_candidate":  "",
				"rejected_candidates": "",
			})).
			WhenAgentRuns().
			Then("the first 3999 bytes stay, because the 4000th falls inside a letter").
			ExpectNoChange().
			ExpectExplanation("## Finding\n\nx" + strings.Repeat("ж", 1993))
	})

	f.RunAgent("no secrets in the explanation", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.2.5").
			GivenInstallationToken("spec-github-token").
			GivenEnv("OPENAI_API_KEY", "spec-openai-key").
			GivenRepositoryWithCommits(1).
			GivenAgentAnswers(answerWith(map[string]string{
				"finding":             "I saw spec-github-token.",
				"selected_candidate":  "",
				"rejected_candidates": "And spec-openai-key.",
			})).
			WhenAgentRuns().
			Then("the explanation stays, the secrets do not").
			ExpectNoChange().
			ExpectExplanation("## Finding\n\nI saw [secret].\n\n## Rejected Candidates\n\nAnd [secret].")
	})

	f.RunAgent("Codex prints its events", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.6.1").
			GivenRepositoryWithCommits(1).
			WhenAgentRuns().
			Then("Codex is asked for JSON events").
			ExpectAgentArgs("exec", "--json")
	})

	f.RunAgent("count the tokens of every turn", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.6.2", "AGENT-3.6.4").
			GivenRepositoryWithCommits(1).
			GivenAgentPrints(
				`{"type":"thread.started","thread_id":"t1"}`,
				`{"type":"turn.completed","usage":{"input_tokens":1000,"cached_input_tokens":800,"output_tokens":50,"reasoning_output_tokens":20}}`,
				`{"type":"item.completed","item":{"type":"agent_message","usage":{"input_tokens":99}}}`,
				`{"type":"turn.completed","usage":{"input_tokens":200,"cached_input_tokens":100,"output_tokens":7}}`,
			).
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the tokens are the sums over the completed turns").
			ExpectNoChange().
			ExpectTokens(contract.TokenUsage{Input: 1200, Cached: 900, Output: 57})
	})

	f.RunAgent("count the tokens of a change", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.6.2").
			GivenRepositoryWithCommits(1).
			GivenAgentPrints(`{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":0,"output_tokens":3}}`).
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the change comes with its tokens").
			ExpectChange("main", "Remove dead code", "cleaned.txt", "clean").
			ExpectTokens(contract.TokenUsage{Input: 10, Output: 3})
	})

	f.RunAgent("count the tokens of a last line without a line break", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.6.2").
			GivenRepositoryWithCommits(1).
			GivenAgentPrints(`{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":0,"output_tokens":3}}`).
			GivenAgentPrintsLast(`{"type":"turn.completed","usage":{"input_tokens":5,"cached_input_tokens":2,"output_tokens":1}}`).
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the last turn counts too").
			ExpectNoChange().
			ExpectTokens(contract.TokenUsage{Input: 15, Cached: 2, Output: 4})
	})

	f.RunAgent("a failed run keeps its tokens", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.6.3").
			GivenRepositoryWithCommits(1).
			GivenAgentPrints(`{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":4,"output_tokens":3}}`).
			GivenAgentExitsWithError().
			WhenAgentRuns().
			Then("the failure comes with the tokens").
			ExpectFailure().
			ExpectTokens(contract.TokenUsage{Input: 10, Cached: 4, Output: 3})
	})

	f.RunAgent("output that is not JSON adds nothing", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.6.4").
			GivenRepositoryWithCommits(1).
			GivenAgentPrints(
				`{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":0,"output_tokens":3}}`,
				`warning: something odd`,
				`{"type":"turn.completed","usage":{"input_tokens":5,"cached_input_tokens":0,"output_tokens":1}}`,
			).
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the run does not fail, and the JSON events still count").
			ExpectNoChange().
			ExpectTokens(contract.TokenUsage{Input: 15, Output: 4})
	})

	f.RunAgent("no events, no tokens", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.6.4").
			GivenRepositoryWithCommits(1).
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the tokens are zero").
			ExpectNoChange().
			ExpectTokens(contract.TokenUsage{})
	})

	f.RunAgent("fail when the agent exits with an error", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.3.1").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentAnswers(answer).
			GivenAgentExitsWithError().
			WhenAgentRuns().
			Then("the run fails and nothing is pushed").
			ExpectFailure().
			ExpectBranches(0, "gc/")
	})

	f.RunAgent("a failed agent gives the reason from its error output", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.3.3").
			GivenRepositoryWithCommits(1).
			GivenAgentFailsWith("ERROR: quota exceeded").
			WhenAgentRuns().
			Then("the run fails with that reason").
			ExpectFailure().
			ExpectFailureReason("ERROR: quota exceeded")
	})

	f.RunAgent("a reason of several lines stays on one line", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.3.3").
			GivenRepositoryWithCommits(1).
			GivenAgentFailsWith("ERROR: quota exceeded\nin the second line\n").
			WhenAgentRuns().
			Then("the line breaks are spaces").
			ExpectFailure().
			ExpectFailureReason("ERROR: quota exceeded in the second line")
	})

	f.RunAgent("a long reason keeps its end", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.3.3").
			GivenRepositoryWithCommits(1).
			GivenAgentFailsWith("the start " + strings.Repeat("x", 1000) + " the end").
			WhenAgentRuns().
			Then("only the last 1000 bytes are in the reason").
			ExpectFailure().
			ExpectFailureReason(" the end").
			ExpectFailureReasonWithout("the start")
	})

	f.RunAgent("a reason keeps the secrets out", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.3.4").
			GivenInstallationToken("spec-github-token").
			GivenEnv("OPENAI_API_KEY", "spec-openai-key").
			GivenRepositoryWithCommits(1).
			GivenAgentFailsWith("bad spec-github-token and spec-openai-key").
			WhenAgentRuns().
			Then("the reason stays, the secrets do not").
			ExpectFailure().
			ExpectFailureReason("bad ").
			ExpectFailureReasonWithout("spec-github-token", "spec-openai-key")
	})

	f.RunAgent("a secret at the cut leaves no piece in the reason", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.3.3", "AGENT-3.3.4").
			GivenEnv("OPENAI_API_KEY", "spec-openai-key").
			GivenRepositoryWithCommits(1).
			GivenAgentFailsWith("spec-openai-key"+strings.Repeat("x", 995)).
			WhenAgentRuns().
			Then("the end of the output stays, and no part of the key does").
			ExpectFailure().
			ExpectFailureReason("xxxxxxxxxx").
			ExpectFailureReasonWithout("key", "spec")
	})

	f.RunAgent("fail when the answer is not JSON", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.3.2").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentAnswers("Done!").
			WhenAgentRuns().
			Then("the run fails and nothing is pushed").
			ExpectFailure().
			ExpectBranches(0, "gc/")
	})

	f.RunAgent("fail when the title is empty", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.3.2").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentAnswers(answerWith(map[string]string{"title": ""})).
			WhenAgentRuns().
			Then("the run fails and nothing is pushed").
			ExpectFailure().
			ExpectBranches(0, "gc/")
	})

	f.RunAgent("kill the agent and its processes when stopped", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.4.1", "AGENT-1.2.1").
			GivenRepositoryWithCommits(1).
			GivenAgentHangsWithChildProcess().
			WhenAgentRunIsStopped().
			Then("no process is left and the folder is gone").
			ExpectFailure().
			ExpectAgentLeftNothing()
	})

	f.RunAgent("a stalled clone does not hold a stopped run", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.4.2").
			GivenRepositoryWithCommits(1).
			GivenCloneStalls().
			WhenRunIsStoppedAfter(time.Second).
			Then("the run ends without an answer from the remote").
			ExpectFailure().
			ExpectAgentNotStarted()
	})

	f.RunAgent("fail when the answer is null", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.5.5").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentAnswers("null").
			WhenAgentRuns().
			Then("the run fails and nothing is pushed, with no panic").
			ExpectFailure().
			ExpectBranches(0, "gc/")
	})

	f.RunAgent("the report reaches the branch when the repository ignores it", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.5.7").
			GivenRepositoryFile(".gitignore", "docs/\n").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the report is committed with the change").
			ExpectReport("dead-code", report)
	})

	f.RunAgent("line breaks at the end of an answer field are dropped", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.5.2", "AGENT-3.5.8").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentAnswers(answerWith(map[string]string{
				"finding":   answerFields["finding"] + "\n",
				"follow_up": answerFields["follow_up"] + "\n\n",
			})).
			WhenAgentRuns().
			Then("the report is laid out as usual").
			ExpectReport("dead-code", report)
	})

	f.RunAgent("a link in place of the report does not reach git settings", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.5.9", "AGENT-4.1.1").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentLinksReportPathsTo("../../.git/config").
			GivenAgentAnswers(answerWith(map[string]string{
				"finding":             "\n[core]\n\tfsmonitor = " + s.CommandPath() + "\n",
				"evidence":            "none",
				"selected_candidate":  "none",
				"rejected_candidates": "none",
				"change":              "none",
				"validation":          "none",
				"risk":                "none",
				"rollback":            "none",
			})).
			WhenAgentRuns().
			Then("the run fails, and nothing the agent chose runs").
			ExpectFailure().
			ExpectBranches(0, "gc/").
			ExpectNoCommandFromTheAgentRan()
	})

	for _, tt := range []struct {
		name  string
		given func(s *extensions.AgentRun)
	}{
		{"a link to a file of the repository", func(s *extensions.AgentRun) { s.GivenAgentLinksReportPathsTo("../../commit-1.txt") }},
		{"a file", func(s *extensions.AgentRun) { s.GivenAgentWritesToReportPaths() }},
		{"a report folder linked inside the working copy", func(s *extensions.AgentRun) {
			s.GivenAgentRuns("mkdir -p elsewhere docs; ln -s ../elsewhere docs/gc-reviews")
		}},
		{"a parent folder linked inside the working copy", func(s *extensions.AgentRun) {
			s.GivenAgentRuns("mkdir -p elsewhere/gc-reviews; ln -s elsewhere docs")
		}},
	} {
		f.RunAgent("something already at the report's path fails the run: "+tt.name, func(s *extensions.AgentRun) {
			s.Covers("AGENT-3.5.9").
				GivenRepositoryWithCommits(1).
				GivenAgentChanges("cleaned.txt", "clean")
			tt.given(s)
			s.GivenAgentAnswers(answer).
				WhenAgentRuns().
				Then("the run fails and nothing is pushed").
				ExpectFailure().
				ExpectBranches(0, "gc/")
		})
	}

	f.RunAgent("a report folder linked outside is not written through", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.5.9").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentRuns("mkdir -p docs; ln -s "+s.OutsidePath()+" docs/gc-reviews").
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the run fails and nothing lands outside the working copy").
			ExpectFailure().
			ExpectBranches(0, "gc/").
			ExpectFolderEmpty("outside")
	})

	const everyHook = "pre-commit prepare-commit-msg commit-msg post-commit post-index-change reference-transaction pre-push"
	for _, tt := range []struct {
		name   string
		script func(s *extensions.AgentRun) string
	}{
		{"core.fsmonitor", func(s *extensions.AgentRun) string { return "git config core.fsmonitor " + s.CommandPath() }},
		{"core.sshCommand with an ssh push address", func(s *extensions.AgentRun) string {
			return "git config core.sshCommand " + s.CommandPath() + "; git config remote.origin.pushurl ssh://git@example.test/x.git"
		}},
		{"an ext push address", func(s *extensions.AgentRun) string {
			return "git config protocol.ext.allow always; git config remote.origin.pushurl ext::" + s.CommandPath()
		}},
		{"a filter", func(s *extensions.AgentRun) string {
			return "git config filter.spec.clean " + s.CommandPath() + "; echo '* filter=spec' > .gitattributes"
		}},
		{"hooks", func(s *extensions.AgentRun) string {
			return "for hook in " + everyHook + "; do cp " + s.CommandPath() + " .git/hooks/$hook; done"
		}},
		{"a hooks folder", func(s *extensions.AgentRun) string {
			return "mkdir .git/agent-hooks; for hook in " + everyHook + "; do cp " + s.CommandPath() + " .git/agent-hooks/$hook; done; git config core.hooksPath .git/agent-hooks"
		}},
	} {
		f.RunAgent("git settings from the agent have no effect: "+tt.name, func(s *extensions.AgentRun) {
			s.Covers("AGENT-4.1.1").
				GivenRepositoryWithCommits(1).
				GivenAgentChanges("cleaned.txt", "clean").
				GivenAgentRuns(tt.script(s)).
				GivenAgentAnswers(answer).
				WhenAgentRuns().
				Then("the change is pushed as usual, and nothing the agent chose runs").
				ExpectChange("main", "Remove dead code", "cleaned.txt", "clean").
				ExpectNoCommandFromTheAgentRan()
		})
	}

	f.RunAgent("a push address from the agent has no effect", func(s *extensions.AgentRun) {
		s.Covers("AGENT-4.1.1").
			GivenRepositoryWithCommits(1).
			GivenAnotherRepositoryOnTheRemote().
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentRuns(`git config remote.origin.pushurl "$(git config remote.origin.url | sed 's#acme/api.git#acme/other.git#')"`).
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the branch goes to the cloned repository and nowhere else").
			ExpectChange("main", "Remove dead code", "cleaned.txt", "clean").
			ExpectNothingPushedToOtherRepository()
	})

	f.RunAgent("a nested repository fails the run", func(s *extensions.AgentRun) {
		s.Covers("AGENT-4.2.1").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentRuns("git init -q vendor/nested && git -C vendor/nested -c user.name=agent -c user.email=agent@example.test commit -q --allow-empty --message nested").
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the run fails and nothing is pushed").
			ExpectFailure().
			ExpectBranches(0, "gc/")
	})

	f.RunAgent("settings of a nested repository run nothing", func(s *extensions.AgentRun) {
		s.Covers("AGENT-4.2.1", "AGENT-4.2.2").
			GivenRepositoryWithCommits(1).
			GivenRepositoryWithGitlink().
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentRuns("rm -rf sub; git init -q sub; git -C sub config core.fsmonitor "+s.CommandPath()).
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the run fails, and nothing the agent chose runs").
			ExpectFailure().
			ExpectBranches(0, "gc/").
			ExpectNoCommandFromTheAgentRan()
	})

	f.RunAgent("a workflow file fails the run", func(s *extensions.AgentRun) {
		s.Covers("AGENT-4.3.1").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges(".github/workflows/steal.yml", "on: push").
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the run fails and nothing is pushed").
			ExpectFailure().
			ExpectBranches(0, "gc/")
	})

	f.RunAgent("a change to an existing workflow fails the run", func(s *extensions.AgentRun) {
		s.Covers("AGENT-4.3.1").
			GivenRepositoryFile(".github/workflows/build.yml", "on: pull_request").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges(".github/workflows/build.yml", "on: push").
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the run fails and nothing is pushed").
			ExpectFailure().
			ExpectBranches(0, "gc/")
	})

	f.RunAgent("a workflow moved out of .github fails the run", func(s *extensions.AgentRun) {
		s.Covers("AGENT-4.3.1").
			GivenRepositoryFile(".github/workflows/build.yml", "on: pull_request").
			GivenRepositoryWithCommits(1).
			GivenAgentRuns(`mkdir ci; mv .github/workflows/build.yml ci/build.yml`).
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the run fails and nothing is pushed").
			ExpectFailure().
			ExpectBranches(0, "gc/")
	})

	f.RunAgent("a change to .gitmodules fails the run", func(s *extensions.AgentRun) {
		s.Covers("AGENT-4.3.2").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges(".gitmodules", "[submodule \"x\"]\n\tpath = x\n\turl = http://example.test/x.git\n").
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the run fails and nothing is pushed").
			ExpectFailure().
			ExpectBranches(0, "gc/")
	})
}
