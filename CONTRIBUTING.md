# Contributing

Please refer to the [Gardener contributor guide](https://gardener.cloud/contribute/contribution-process/contributor-guide/) for the general contribution process (DCO, license headers, CLA, etc.).

The guidelines below cover contribution expectations specific to this repository.

---

## Opening a PR

- Ensure there's an issue before raising a PR.
- Check whether the implementation PR should be preceded by a design proposal — especially when there are multiple possible designs/solutions, or the feature has end-user-facing impact. If so, note this in the issue, and once the proposal PR is raised, link it from the issue.
- Write a detailed PR description: what the change does, why it's needed, and anything a reviewer needs to know. Follow the template structure defined for PRs so that important points are not missed out.
- Keep the PR description and release notes updated during the review process to ensure the latest changes are captured and the release notes are correct.
- Keep commits logically grouped rather than bundling large, unrelated changes into one commit. A series of small, coherent commits is much easier to review than one big diff.
- The commit history should not contain merge commits. Use **Squash and merge** when merging PRs to the `master` branch. This combines all commits from the PR into a single commit, keeping the `master` branch history clean and linear. The PR title becomes the commit message on `master`, so ensure it's clear and descriptive.
- Write commit messages that clearly state what changed in that commit, not just a generic summary of the PR. See:
  - [conventionalcommits.org](https://www.conventionalcommits.org/en/v1.0.0)
  - [refactoringenglish.com/excerpts/commit-messages](https://refactoringenglish.com/excerpts/commit-messages)
- Add automated tests for your changes (unit or integration as appropriate). List them in the PR description so reviewers know what coverage you've added. Automated tests reduce the effort of repeated manual verification by maintainers.
- If manual testing is necessary, document it in the PR description: what you tested, how you tested it, and the result. Also explain **why** it cannot be automated — this helps reviewers understand the testing limitations and informs future test infrastructure improvements.
- If the PR is not ready for review, open it as a draft. Convert to ready only when you want eyes on it.
- Keep the PR focused on one thing. Fix unrelated issues in a separate PR rather than bundling them.
- If your PR depends on another PR being merged first, say so clearly in the PR description and link to it.
- If LLMs are used for assistance, do your own diligent reviews — don't leave it to PR reviewers to validate the generated output.


---

## PR Review Process

### For PR Authors

- When you address a review comment, reply on the thread referencing the commit that contains the change, and briefly explain what was changed. This tells the reviewer the comment is ready for re-review and narrows down exactly what to look at.
- If you addressed a comment differently than suggested, or decided not to act on it, say so explicitly and explain why.
- If a comment was addressed through an offline discussion (chat, call, etc.), summarize the outcome in the PR thread so the reasoning is visible to anyone reading the PR later.
- Do not mark a reviewer's comment as **Resolved** yourself. Resolving is the reviewer's call, since it gives them the chance to look at the change before the thread is closed.
- Do not leave a comment unaddressed. Reviewers are taking time to review your PR carefully; respond to every comment thoughtfully and with general courtesy.
- Once you have addressed all review comments, explicitly request a re-review from the reviewer. Do not assume they will check back on their own.
- No matter how small/trivial the PR is, do not LGTM or approve your own PRs, or PRs you've pushed commits to.

### For Reviewers

- Explain requested changes clearly — what you want changed and why. This reduces back-and-forth and keeps threads short.
- Review the PR description, release notes and commit messages, not just the code diff. If any of these is unclear or missing context, ask for it to be improved before approving.
- If you start a review, see it through. Leaving a PR in "changes requested" and then going silent blocks the author and stalls the PR.
- Once a comment has been addressed, resolve it.
- Once all requested changes have been addressed and you're satisfied with the PR, make sure to add an `/lgtm`. This on its own will not cause the PR to be merged unless there is already a `/approved` present.
- Test all changes, not just the ones that introduce new tests. Run the relevant automated tests (unit, integration) to confirm they pass reliably. Repeat any manual testing performed by the author to independently verify the results; if manual testing was documented, validate that the described outcome matches what you observe.
- If the PR introduces new tests, also check them for flakiness — a single passing run is not sufficient. Use a stress tool such as [`golang.org/x/tools/cmd/stress`](https://pkg.go.dev/golang.org/x/tools/cmd/stress) to run them repeatedly. Verify the tests are not doing unnecessary logging and provide meaningful coverage. This validates that the tests actually work and catches environment/provider-specific issues the author may not have encountered.
- See also:
  - [go.dev/wiki/CodeReviewComments](https://go.dev/wiki/CodeReviewComments)
  - [google.github.io/eng-practices — What to look for in a code review](https://google.github.io/eng-practices/review/reviewer/looking-for.html)

---

The guidelines above keep the review conversation useful on its own. Anyone revisiting the PR later can see what changed, why, and confirm every comment was actually addressed.

---

## Coding Guidelines

Complexity reduction is the easiest way to keep the codebase maintainable over time, and the best way to reduce complexity is by decomposing the problem into manageable chunks of code. Some things to keep in mind when thinking about approaching a problem:
- Try to keep things functional. This means ensuring that the problem decomposition leads to independently composable functions. This reduces the coupling (dependencies) between functions.
- A simple way to gauge if the approach is getting too complex is to look for excessive redirection in the functions. Deep functions are a sign that there's a lot of intertwining, which leads to a higher cognitive burden [^1] on the maintainer.
- Dependency exists if a given piece of code cannot be understood and modified in isolation. In other words, it is tied to other pieces of code and modification requires changes at multiple points. [^2]

As a guideline, some pointers to keep in mind while developing any feature/bugfix:
- Simplicity is key. Simple code is easier to maintain (revisit) and easier to reason about. However, coming up with simple solutions is not always easy. [^3]
- In an effort to work out the issue, it's possible to get carried away and come up with clever solutions that are extremely elegant but difficult to reason about. Writing clever code should not be the focus. Clever is not simple.
- Another way to keep your future self thankful while developing is to have clear, concise descriptive names. It's possible to be lazy and come up with bad placeholder names, but good names indicate clarity of design in the developer's mind. [^4]
- Keep an eye out for *code smells*. This should not be deferred to the review stage but be actively done by the PR author during the development phase itself. [^5]
  > Code smells are structures in the code that suggest (sometimes scream for) the possibility of refactoring. They are software design patterns common to bad code.

However, note that the above are just suggestions and not maxims to be religiously followed. Sometimes, an approach that requires more lines of code might actually be simpler if it reduces the cognitive load. The goal should not be to follow these guidelines at all times. The goal is to reduce complexity while solving the problem at hand.

[^1]: [Cognitive load](https://minds.md/zakirullin/cognitive) is what matters - A very nicely written document to understand the harms of complexity. Well worth a read.
[^2]: An excellent guide to software development by the co-author of the Raft consensus algorithm, John Ousterhout: [A philosophy of Software Design](https://books.danielhofstetter.com/a-philosophy-of-software-design/). This links to a review of the book that collates all the key points and summarizes it in a nice fashion.
[^3]: Rich Hickey's (Author of Clojure) wonderful talk titled [Simple Made Easy](https://github.com/matthiasn/talk-transcripts/blob/master/Hickey_Rich/SimpleMadeEasy.md)
[^4]: [Naming things - Tigerstyle](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md#naming-things): Do note that this guide is written with their `zig` codebase in mind, so suggestions of using `snake_case` etc might not be applicable for a `go` program. However, the general considerations discussed while naming things are still relevant.
[^5]: Defined by Martin Fowler in the quintessential book "Refactoring: Improving the Design of Existing Code". [Code smells - Fowler](https://martinfowler.com/bliki/CodeSmell.html)

