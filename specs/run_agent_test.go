package specs

import (
	"testing"

	"github.com/Andrew-Sem/gc/specs/extensions"
)

const answer = `{"title":"Remove dead code","description":"The helper has no callers."}`

func TestRunAgent(t *testing.T) {
	f := extensions.NewFeature(t, "run-agent.md")

	f.RunAgent("hand over a change", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.1.1", "AGENT-3.1.2").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the change is on a new branch and main is untouched").
			ExpectChange("main", "Remove dead code", "The helper has no callers.", "cleaned.txt", "clean")
	})

	f.RunAgent("clone and push with the GitHub token", func(s *extensions.AgentRun) {
		s.Covers("AGENT-1.1.3", "AGENT-3.1.4").
			GivenEnv("GITHUB_TOKEN", "another-github-token").
			GivenRepositoryWithCommits(1).
			GivenAgentChanges("cleaned.txt", "clean").
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the repository saw that token on the clone and on the push").
			ExpectGitAuthenticated("another-github-token")
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

	f.RunAgent("keep secrets away from the agent", func(s *extensions.AgentRun) {
		s.Covers("AGENT-2.2.1", "AGENT-2.2.3").
			GivenEnv("GITHUB_TOKEN", "spec-github-token").
			GivenEnv("GC_WEBHOOK_SECRET", "spec-webhook-secret").
			GivenEnv("OPENAI_API_KEY", "spec-openai-key").
			GivenRepositoryWithCommits(1).
			WhenAgentRuns().
			Then("only the OpenAI key reaches the agent").
			ExpectAgentEnvWithout("GITHUB_TOKEN", "GC_WEBHOOK_SECRET").
			ExpectAgentEnv("CODEX_API_KEY", "spec-openai-key")
	})

	f.RunAgent("use a local proxy instead of OpenAI", func(s *extensions.AgentRun) {
		s.Covers("AGENT-2.1.3").
			GivenEnv("OPENAI_BASE_URL", "http://127.0.0.1:8317/v1").
			GivenRepositoryWithCommits(1).
			WhenAgentRuns().
			Then("the agent is told the address").
			ExpectAgentArgs("-c", `openai_base_url="http://127.0.0.1:8317/v1"`)
	})

	f.RunAgent("use OpenAI without a base URL", func(s *extensions.AgentRun) {
		s.Covers("AGENT-2.1.3").
			GivenUnset("OPENAI_BASE_URL").
			GivenRepositoryWithCommits(1).
			WhenAgentRuns().
			Then("the agent is told no address").
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
			GivenEnv("GITHUB_TOKEN", "spec-github-token").
			GivenRepositoryWithCommits(1).
			WhenAgentRuns().
			Then("the token is nowhere in the folder the agent sees").
			ExpectWorkingCopyWithout("spec-github-token")
	})

	f.RunAgent("no change when the agent changed nothing", func(s *extensions.AgentRun) {
		s.Covers("AGENT-3.2.1").
			GivenRepositoryWithCommits(1).
			GivenAgentAnswers(answer).
			WhenAgentRuns().
			Then("the run returns no change").
			ExpectNoChange().
			ExpectBranches(0, "gc/")
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
			GivenAgentAnswers(`{"title":"","description":"The helper has no callers."}`).
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
}
