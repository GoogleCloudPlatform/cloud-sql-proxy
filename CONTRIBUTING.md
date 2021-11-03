# Contributing

1. **Please sign one of the contributor license agreements below!**
1. Fork the repo, develop and test your code changes, add docs.
1. Make sure that your commit messages clearly describe the changes.
1. Send a pull request.

## Table of contents
* [Opening an issue](#opening-an-issue)
* [How to run tests](#how-to-run-tests)
* [Contributor License Agreements](#contributor-license-agreements)
* [Contributor Code of Conduct](#contributor-code-of-conduct)

## Opening an issue

If you find a bug in the proxy code or an inaccuracy in the documentation,
please open an issue. GitHub provides a guide, [Mastering
Issues](https://guides.github.com/features/issues/), that is useful if you are
unfamiliar with the process. Here are the specific steps for opening an issue:

1. Go to the project issues page on GitHub.
1. Click the green `New Issue` button located in the upper right corner.
1. In the title field, write a single phrase that identifies your issue.
1. In the main editor, describe your issue.
1. Click the submit button.

Thank you. We will do our best to triage your issue within one business day, and
attempt to categorize your issues with an estimate of the priority and issue
type. We will try to respond with regular updates based on its priority:

* **Critical** respond and update daily, resolve with a week
* **High** respond and update weekly, resolve within six weeks
* **Medium** respond and update every three months, best effort resolution
* **Low** respond and update every six months, best effort resolution

The priority we assign will be roughly a function of the number of users we
expect to be impacted, as well as its severity. As a rule of thumb:

<table>
  <thead>
    <tr>
      <th rowspan="2">Severity</th>
      <th colspan="4">Number of users</th>
    </tr>
    <tr>
      <th>Handful</th>
      <th>Some</th>
      <th>Most</th>
      <th>All</th>
    </tr>
  </thead>
  <tr>
    <td>Easy, obvious workaround</td>
    <td>Low</td>
    <td>Low</td>
    <td>Medium</td>
    <td>High
  </tr>
  <tr>
<td>Non-obvious workaround available</td>
<td>Low</td>
<td>Medium</td>
<td>High</td>
<td>Critical</td>
  </tr>

  <tr>
<td>Functionality blocked</td>
<td>High</td>
<td>High</td>
<td>Critical</td>
<td>Critical</td>
  </tr>
</table>

## How to run tests

The test suite includes both unit and integration tests. For macOS and Linux,
there is a depenency on [FUSE][] that must be present on the system.

### Test Dependencies

When running tests on macOS and Linux, users will need to first install
[FUSE][]. Windows users may skip this step.

On Debian, use:

```
sudo apt-get install fuse
```

On macOS, use:

```
brew install --cask macfuse
```

### How to run just unit tests

```
go test -short ./...
```

### How to run all tests

To run all integration tests, users will need a Google Cloud project with a
MySQL, PostgreSQL, and SQL Server database, in addition to installing FUSE
support. Note: Pull Requests will run these tests and as a result may be skipped
locally if necessary.

A sample `.envrc.example` file is included in the root directory which documents
which environment variables must be set to run the integration tests. Copy this
example file to `.envrc` at the root of the project, supply all the correct
values for each variable, source the file (`source .envrc`, or consider using
[direnv][]), and then run:

```
go test ./...
```

## Contributor License Agreements

Open-source software licensing is a wonderful arrangement that benefits
everyone, but in an imperfect world, we all need to exercise some legal
prudence. In order to protect you, Google, and most of all, everyone who comes
to depend on these libraries, we require that all contributors sign our short
and human-readable Contributor License Agreement (CLA). We don't want to open
the door to patent trolls, predatory lawyers, or anyone else who isn't on board
with creating value and making the world a better place. We hope you will agree
that the CLA offers very important protection and is easy to understand. Take a
moment to read it carefully, and if you agree with what you read, please sign it
now. If you believe you've already signed the appropriate CLA already for this
or any other Google open-source project, you shouldn't have to do so again. You
can review your signed CLAs at
[cla.developers.google.com/clas](https://cla.developers.google.com/clas).

First, check that you are signed in to a [Google
Account](https://accounts.google.com) that matches your [local Git email
address](https://help.github.com/articles/setting-your-email-in-git/). Then
choose one of the following:

* If you are **an individual writing original source code** and **you own the
  intellectual property**, sign the [Individual
  CLA](https://developers.google.com/open-source/cla/individual).
* If you work for **a company that wants to allow you to contribute**, sign the
  [Corporate CLA](https://developers.google.com/open-source/cla/corporate).

You (and your authorized signer, if corporate) can sign the CLA
electronically. After that, we'll be able to accept your contributions.

## Contributor Code of Conduct

As contributors and maintainers of this project, and in the interest of
fostering an open and welcoming community, we pledge to respect all people who
contribute through reporting issues, posting feature requests, updating
documentation, submitting pull requests or patches, and other activities.

We are committed to making participation in this project a harassment-free
experience for everyone, regardless of level of experience, gender, gender
identity and expression, sexual orientation, disability, personal appearance,
body size, race, ethnicity, age, religion, or nationality.

Examples of unacceptable behavior by participants include:

* The use of sexualized language or imagery
* Personal attacks
* Trolling or insulting/derogatory comments
* Public or private harassment
* Publishing other's private information, such as physical or electronic
addresses, without explicit permission
* Other unethical or unprofessional conduct.

Project maintainers have the right and responsibility to remove, edit, or reject
comments, commits, code, wiki edits, issues, and other contributions that are
not aligned to this Code of Conduct.  By adopting this Code of Conduct, project
maintainers commit themselves to fairly and consistently applying these
principles to every aspect of managing this project.  Project maintainers who do
not follow or enforce the Code of Conduct may be permanently removed from the
project team.

This code of conduct applies both within project spaces and in public spaces
when an individual is representing the project or its community.

Instances of abusive, harassing, or otherwise unacceptable behavior may be
reported by opening an issue or contacting one or more of the project
maintainers.

This Code of Conduct is adapted from the [Contributor
Covenant](http://contributor-covenant.org), version 1.2.0, available at
[http://contributor-covenant.org/version/1/2/0/](http://contributor-covenant.org/version/1/2/0/)

[FUSE]: https://www.kernel.org/doc/html/latest/filesystems/fuse.html
[direnv]: https://direnv.net/
