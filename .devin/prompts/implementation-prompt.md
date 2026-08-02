# Implementation Prompt for OwnWave v2 Build Tracker

Use this prompt when asking an AI to implement the next open issue from the ordered build tracker.

- Replace `[ISSUE_NUMBER]` with a specific GitHub issue number if you know it.
- Or leave the placeholder in place and the AI will pick the next unblocked, lowest-order open issue from the issue comments.

---

**Implement the next issue from the ordered build tracker**

You are a senior full-stack developer. The repo is `FelixClements/OwnWave` and the main spec is `PLAN.md`. The human is not a developer, so you do all the coding. Do not ask them to write code, but do ask them one focused question at a time if a decision is unclear.

Task:

1. **Pick the issue**
   - List open issues with `gh issue list --state open --limit 100`.
   - Use the "Implementation order: N/27" comments to find the next unblocked, lowest-order issue.
   - If `[ISSUE_NUMBER]` is given, open that issue directly.
   - Read the issue title and body to understand what needs to be built.

2. **Read the spec and context**
   - Read `PLAN.md`.
   - Read the closed wayfinder map `#1` for the original decision context.
   - Read any code files that are clearly relevant.

3. **Design before coding**
   - If the issue is small and clear, just implement it.
   - If it touches architecture or could have multiple approaches, briefly explain your plan and wait for the human's "ok" before continuing.

4. **Implement**
   - Follow the existing code style and conventions in the repo.
   - Use the same languages/frameworks already in use (Go, Python, Next.js/TypeScript, Postgres, Docker).
   - Write tests if the repo has test infrastructure; if not, add minimal verification steps to `docker compose` or a `justfile` recipe.
   - The v1 foundational tooling is now in place (`justfile`, `lib/api.ts`, `golang-migrate`, env handling), so build on that unless the issue is specifically about one of those.

5. **Verify**
   - Run `just build` to make sure all images still build.
   - If there is a relevant `just test`, `just scan`, `just db-migrate`, or `docker compose up --build` command, run it.
   - For streaming or crossfade issues, run the relevant v2 verification step from `PLAN.md`.

6. **Commit**
   - Write a concise commit message in the repo's existing style.
   - Include "Generated with [Devin](https://devin.ai)" if you use Devin.
   - Do **not** push to `origin/main`.

7. **Report back**
   - Summarize what changed.
   - Tell the human the commit hash.
   - Explain how to manually test it.
   - Ask if they want you to push, open a PR, or continue with the next issue in the ordered build tracker.

---

## Quick example

> Implement the next ordered issue in `FelixClements/OwnWave`. Read the issue, `PLAN.md`, and wayfinder map `#1`. Follow the repo's existing conventions, verify with `just build` (and any relevant `just` recipe), commit, and report back. Do not push.

Or for a specific issue:

> Implement issue `#24` in `FelixClements/OwnWave`. Read the issue, `PLAN.md`, and wayfinder map `#1`. Follow the repo's existing conventions, verify with `just build` and `docker compose up --build`, commit, and report back. Do not push.
