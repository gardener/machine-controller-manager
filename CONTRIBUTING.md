# Contributing

Please refer to the [Gardener contributor guide](https://github.com/gardener/documentation/blob/master/CONTRIBUTING.md) for the general contribution process (DCO, license headers, CLA, etc.).

The guidelines below cover contribution expectations specific to this repository.

---

## Opening a PR

- If the problem being solved is non-trivial, ensure there's an issue before raising a PR. If the design/implementation is involved, document it as part of issue triaging.
- Write a detailed PR description: what the change does, why it's needed, and anything a reviewer needs to know. Organize it so the important parts (motivation, approach, anything risky) are easy to find rather than buried in one long paragraph.
- Keep commits logically grouped rather than bundling large, unrelated changes into one commit. Reviewing a series of small, coherent commits is much easier than reviewing one big diff.
- Write commit messages that clearly state what changed in that commit, not just a generic summary of the PR. See:
  - [conventionalcommits.org](https://www.conventionalcommits.org/en/v1.0.0)
  - [refactoringenglish.com/excerpts/commit-messages](https://refactoringenglish.com/excerpts/commit-messages)
- Note any manual testing in the PR description (what you tested, how, and the result), and keep it updated as the PR evolves.

---

## PR Review Process

### For PR Authors

- Simple is not easy; clever is not simple. Keep the code simple and readable.
- Decouple things, keep things functional (compose, don't complect), and reduce cognitive burden by minimizing layers of indirection — functions shouldn't be too deep, nor too large.
- If LLMs are used for assistance, do your own diligent reviews — don't leave it to PR reviewers to validate the generated output.
- If the feature is non-trivial or has an end-user facing impact/use-case, the PR implementation should ideally be preceded by a design proposal.
- If the PR is not ready for review, open it as a draft. Convert to ready only when you want eyes on it.
- Keep the PR focused on one thing. Fix unrelated issues in a separate PR rather than bundling them.
- If your PR depends on another PR being merged first, say so clearly in the description and link to it.
- When you address a review comment, reply on the thread referencing the commit that contains the change, and briefly explain what was changed. This tells the reviewer the comment is ready for re-review and narrows down exactly what to look at.
- If you addressed a comment differently than suggested, or decided not to act on it, say so explicitly and explain why.
- If a comment was addressed through an offline discussion (chat, call, etc.), summarize the outcome in the PR thread so the reasoning is visible to anyone reading the PR later.
- Do not mark a reviewer's comment as **Resolved** yourself. Resolving is the reviewer's call, since it gives them the chance to look at the change before the thread is closed.
- Do not leave a comment both unresolved and unaddressed. Reviewers are taking time to review your PR carefully; respond to every comment thoughtfully and with general courtesy.
- Once you have addressed all review comments, explicitly request a re-review from the reviewer. Do not assume they will check back on their own.

### For Reviewers

- Explain requested changes clearly — what you want changed and why. This reduces back-and-forth and keeps threads short.
- Review the PR description and commit messages, not just the code diff. If either is unclear or missing context, ask for it to be improved before approving.
- If you start a review, see it through. Leaving a PR in "changes requested" and then going silent blocks the author and stalls the PR.
- Once a comment has been addressed, resolve it.
- Once all requested changes have been addressed and you're satisfied with the PR, approve it with `/lgtm`.
- If the PR introduces tests, check for flakiness, try to run them locally, and verify they aren't doing unnecessary logging or too little.
- See also:
  - [go.dev/wiki/CodeReviewComments](https://go.dev/wiki/CodeReviewComments)
  - [google.github.io/eng-practices — What to look for in a code review](https://google.github.io/eng-practices/review/reviewer/looking-for.html)


This keeps the review conversation useful on its own: anyone revisiting the PR later can see what changed, why, and confirm every comment was actually closed rather than approved without being fully addressed.






