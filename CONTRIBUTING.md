# Contributing to Volplugin

Patches and feature additions are totally welcome to volplugin! For now, please
see the [README](https://github.com/contiv/volplugin/blob/master/README.md) for
build and test instructions.

There are other ways to contribute, not just code and feature additions
- report issues, proposing documentation and design changes, submit bug
fixes, propose design changes, discuss use cases or become a maintainer.

## Submitting pull requests to change the documentation or the code

Changes can be proposed by sending a pull request (PR). A maintainer
will review the changes and provide feedback.

The pull request will be merged into the master branch after discussion.

Please make sure to run the tests and that the tests pass before
submitting the PR. Please keep in mind that some changes might not be
merged if the maintainers decide they can't be merged.

Please squash your commits to one commit per fix or feature. The resulting
commit should have a single meaningful message.

## Commit message guidelines

The commit should have a short summary on the first line. The description
should use verbs in the imperative (e.g.`librbd: change a widget`, not
`librbd: changed a widget`). The second line should be left empty.

The short summary should include the name of the directory affected by
the commit (e.g.: `librbd: change a widget`).

A longer description of what the commit does should start on the third
line when such a description is deemed necessary. Paragraphs following
this line should be left empty.

If you have trouble with the appropriate git commands to handle these
requirements, please let us know! We're happy to help.

## Legal Stuff: Sign your work
You must sign off on your work by adding your signature at the end of the
commit message. Your signature certifies that you wrote the patch or
otherwise have the right to pass it on as an open-source patch.
By signing off your work you ascertain following (from [developercertificate.org](http://developercertificate.org/)):

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
660 York Street, Suite 102,
San Francisco, CA 94110 USA

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

Every git commit message must have the following at the end on a separate line:

    Signed-off-by: Joe Smith <joe.smith@email.com>

Your real legal name has to be used. Anonymous contributions or contributions
submitted using pseudonyms cannot be accepted.

Two examples of commit messages with the sign-off message can be found below:
```
netmaster: fix bug

This fixes a random bug encountered in netmaster.

Signed-off-by: Joe Smith <joe.smith@email.com>
```
```
netmaster: fix bug

Signed-off-by: Joe Smith <joe.smith@email.com>
```

If you set your `user.name` and `user.email` git configuration options, you can
sign your commits automatically with `git commit -s`.

These git options can be set using the following commands:
```
git config user.name "Joe Smith"
git config user.email joe.smith@email.com
```

`git commit -s` should be used now to sign the commits automatically, instead of
`git commit`.

## Environment

We currently test against the docker [master branch](https://master.dockerproject.org)
and this will be downloaded on `make start` to be injected into the VMs by ansible.

## Building VM Images

Tools and configurations for building the VM images can be found in the [build repository](https://github.com/contiv/build).

## System Tests

Our system tests are an integration suite that runs from the host, unlike the
unit tests which run on the guest(s). As a result system tests have unique
requirements. The system tests also are responsible for setting up volplugin,
volmaster, etcd and other dependencies.

* Must run on all (reasonably supported) platforms for development, e.g.:
  * Mac OS X
  * Linux (ubuntu and centos)
  * Windows *should* work :)
* Each test is responsible for the test setup and teardown of each resource
  * see the `rebootstrap()` function in the system tests (and the tests
    themselves for usage explanations).
  * there are also numerous utility functions in [systemtests/util_test.go](https://github.com/contiv/volplugin/blob/master/systemtests/util_test.go)
    which should be used in any test.
