# Unikorn

A badass, opinionated, deployer of souls!

![Unikorn](https://i.stack.imgur.com/EzZiD.png)

## Installation

For now, you can try:

```
go install git@github.com:echercloudai/unikorn/cmd/unikornctl
```

Add `$(go env GOPATH)/bin` to your PATH.
Run `unikornctl --help` and feast upon the builtin documentation.

However... propper release binaries will be via a git clone, and:

```
make
```

That will do other magical stuff like populating version information from git/make, that are the sources of truth.
