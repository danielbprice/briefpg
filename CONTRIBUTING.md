We welcome additional contributions subject to our discretion about scope
and appropriateness.  If you're thinking of contributing, here's what you
should know.

# Overview

 1. Please read and understand the Code of Conduct for this project,
    available in the project directory.

 2. Check to make sure if an issue for the problem you're addressing,
    or feature you're adding, has already been filed.  If not, file one here:

	https://github.com/danielbprice/briefpg/issues

    Please indicate in the description of the issue that you're working on
    the issue, so we don't duplicate effort.

 3. By submitting code to the project, you are asserting that the work is
    either your own, or that you are authorized to submit it to this project.
    Further, you are asserting that the project may continue to use, modify,
    and redistribute your contribution under the terms in the LICENSE file.

# Nitty Gritty

 1. We maintain an "always release ready" stance for the master branch. That
    is, at any point in time the tree should be in a state that a release
    could be cut, and bisect should never find a point where an issue is
    incompletely fixed or addressed.

 2. All code must pass go vet, and be go fmt compliant. We also try to pass
    the default set of
    [golangci-lint](https://github.com/golangci/golangci-lint) checks cleanly,
    although this can be a moving target.  We support at least the last two
    minor versions of Golang, and we desire to support the last three to four.

 3. New features should have tests where possible, and the existing tests must
    continue to pass.  We use the go test framework.

 4. Every issue must be fixed by at most one git commit, which shall normally
    be identified in the first line of the commit message using the syntax

	"fixes #<issue#> <exact issue synopsis>"

    You can have multiple such lines if your commit addresses multiple issues,
    but this is normally discouraged.

 5. No merge commits.  Rebase if you need to.

 6. Additional text may follow the above line(s), separated from them by an
    empty line.  Normally this is not necessary, since the information should
    be in the bug tracking system.

 7. Submit a github pull request.  Ideally just one bug per PR if possible,
    and based upon the latest commit in the github master branch.

 8. We may rebase your changes, including squashing multiple commits,
    or ask you to do so, if you have not followed the procedure above, or
    if other changes have been made to the tree since you committed.

## Attribution

This contribution guide was derived from the one used by the
[nanomsg](https://github.com/nanomsg/nanomsg) project.