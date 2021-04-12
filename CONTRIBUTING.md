# Contributing

Thank you for your interest in contributing to Chain! The goal of the chain-main repository is to develop the implementation
of Crypto.org Chain to best power its network use cases in payments, finance and digital assets.
Good places to start are this document and [the official documentation](https://github.com/crypto-org-chain/chain-docs). If you have any questions, feel free to ask on [Discord](https://discord.gg/pahqHz26q4).

All work on the code base tries to adhere to the "Development Process" described in [The Collective Code Construction Contract (C4)](https://rfc.zeromq.org/spec/42/#24-development-process).

## Code of Conduct

All contributors are expected to follow our [Code of Conduct](CODE_OF_CONDUCT.md).

## Requests involving no code changes (e.g. conventions, processes or network parameters)

Requests involving no code changes (e.g. conventions, processes or network parameters) should be posted as [Github Discussions](https://github.com/crypto-org-chain/chain-main/discussions) threads to allow for early feedback before e.g. being submitted for formal network governance decisions.

## Feature requests and bug reports

Feature requests and bug reports should be posted as [Github issues](https://github.com/crypto-org-chain/chain-main/issues/new/choose).
In an issue, please describe what you did, what you expected, and what happened instead.

If you think that you have identified an issue with Chain that might compromise
its users' security, please do not open a public issue on GitHub. Instead,
we ask you to refer to [security policy](SECURITY.md).

## Consensus-breaking and large structural code changes

When the issue is well understood but the solution leads to a consensus-breaking change (i.e. a need for the network-wide upgrade coordination) or large structural changes to the code base, these changes should be proposed in the form of an Architectural Decision Record
([ADR](https://github.com/crypto-org-chain/chain-main/blob/master/docs/architecture/README.md)). The ADR will help build consensus on an overall strategy to ensure the code base maintains coherence in the larger context. If you are not comfortable with writing an ADR, you can open a less-formal issue and the maintainers will help you turn it into an ADR.

## Working on issues
There are several ways to identify an area where you can contribute to Chain:

* You can reach out by sending a message in the developer community communication channel, either with a specific contribution in mind or in general by saying "I want to help!".
* Occasionally, some issues on Github may be labelled with `help wanted` or `good first issue` tags.

We use the variation of the "fork and pull" model where contributors push changes to their personal fork and create pull requests to bring those changes into the source repository.
Changes in pull requests should satisfy "Patch Requirements" described in [The Collective Code Construction Contract (C4)](https://rfc.zeromq.org/spec:42/C4/#23-patch-requirements). The code and comments should follow [Effective Go Guide](https://golang.org/doc/effective_go.html) (and [Uber Style Guide](https://github.com/uber-go/guide/blob/master/style.md)). Many of the code style rules are captured by `go fmt`, `golint`, `go vet` and other tools, so we recommend [setting up your editor to do formatting and lint-checking for you](https://github.com/golang/go/wiki/IDEsAndTextEditorPlugins).

Once you identified an issue to work on, this is the summary of your basic steps:

* Fork Chain's repository under your Github account.

* Clone your fork locally on your machine.

* Post a comment in the issue to say that you are working on it, so that other people do not work on the same issue.

* Create a local branch on your machine by `git checkout -b branch_name`.

* Commit your changes to your own fork -- see [C4 Patch Requirements](https://rfc.zeromq.org/spec:42/C4/#23-patch-requirements) for guidelines.

* Include tests that cover all non-trivial code.

* Check you are working on the latest version on master in Chain's official repository. If not, please pull Chain's official repository's master (upstream) into your fork's master branch, and rebase your committed changes or replay your stashed changes in your branch over the latest changes in the upstream version.

* Run all tests locally and make sure they pass.

* If your changes are of interest to other developers, please make corresponding changes in the official documentation and the changelog.

* Push your changes to your fork's branch and open the pull request to Chain's repository master branch.

* In the pull request, complete its checklist, add a clear description of the problem your changes solve, and add the following statement to confirm that your contribution is your own original work: "I hereby certify that my contribution is in accordance with the Developer Certificate of Origin (https://developercertificate.org/)."

* The reviewer will either accept and merge your pull request, or leave comments requesting changes via the Github PR interface (you should then make changes by pushing directly to your existing PR branch).

### Changelog

Every non-trivial PR must update the [CHANGELOG.md](./CHANGELOG.md).

The Changelog is *not* a record of what Pull Requests were merged;
the commit history already shows that. The Changelog is a notice to the user
about how their expectations of the software should be modified. 
It is part of the UX of a release and is a *critical* user facing integration point.
The Changelog must be clean, inviting, and readable, with concise, meaningful entries. 
Entries must be semantically meaningful to users. If a change takes multiple
Pull Requests to complete, it should likely have only a single entry in the
Changelog describing the net effect to the user.

When writing Changelog entries, ensure they are targeting users of the software,
not fellow developers. Developers have much more context and care about more
things than users do. Changelogs are for users. 

Changelog structure is modeled after 
[Tendermint
Core](https://github.com/tendermint/tendermint/blob/master/CHANGELOG.md)
and 
[Hashicorp Consul](http://github.com/hashicorp/consul/tree/master/CHANGELOG.md).
See those changelogs for examples.

Changes for a given release should be split between the five sections: Security, Breaking
Changes, Features, Improvements, Bug Fixes.

Changelog entries should be formatted as follows:
```
- [pkg] \#xxx Some description about the change (@contributor)
```
Here, `pkg` is the part of the code that changed (typically a
top-level crate, but could be <crate>/<module>), `xxx` is the pull-request number, and `contributor`
is the author/s of the change.

It's also acceptable for `xxx` to refer to the relevant issue number, but pull-request
numbers are preferred.
Note this means pull-requests should be opened first so the changelog can then
be updated with the pull-request's number.

Changelog entries should be ordered alphabetically according to the
`pkg`, and numerically according to the pull-request number.

Changes with multiple classifications should be doubly included (eg. a bug fix
that is also a breaking change should be recorded under both).

Breaking changes are further subdivided according to the APIs/users they impact.
Any change that effects multiple APIs/users should be recorded multiply - for
instance, a change to some core protocol data structure might need to be
reflected both as breaking the core protocol but also breaking any APIs where core data structures are
exposed.

## Releases

Our release process is as follows:

1. The [changelog](#changelog) is updated to reflect and summarize all changes in
   the release.
2. If the changes contain consensus-breaking changes, a new `release/vX` (or `release/vX-testnet`) branch is created
   with the previously agreed upgrade handler code.
   If not, changes are cherry-picked from the trunk/master onto the existing `release/vX` (or `release/vX-testnet`) branch. (See [trunk-based development](https://trunkbaseddevelopment.com/branch-for-release/).)
3. The target commit on the release branch is tagged with `vX.Y.Z` according to the version number of
   the anticipated release (e.g. `v1.0.0`).
4. The tag commit push will trigger the CI pipeline -- if the build passes, Go Releaser will publish the respective binaries.
5. The GitHub release notes are updated according to the changelog, potentially with pointers to the relevant upgrade documentation.

### Developer Certificate of Origin
All contributions to this project are subject to the terms of the Developer Certificate of Origin, available [here](https://developercertificate.org/) and reproduced below:

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
1 Letterman Drive
Suite D4700
San Francisco, CA, 94129

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.

Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```    
