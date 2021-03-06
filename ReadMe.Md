![](images/jabba.png)

Achuta! Jabba is a json friendly reverse proxy for APIs, written in golang.

[![Circleci Builds](https://circleci.com/gh/simonmittag/jabba.svg?style=shield)](https://circleci.com/gh/simonmittag/jabba)
[![Github Workflows](https://github.com/simonmittag/jabba/workflows/Go/badge.svg)](https://github.com/simonmittag/jabba/actions)
[![Github Issues](https://img.shields.io/github/issues/simonmittag/jabba)](https://github.com/simonmittag/jabba/issues)
[![Github Activity](https://img.shields.io/github/commit-activity/m/simonmittag/jabba)](https://img.shields.io/github/commit-activity/m/simonmittag/jabba)  
[![Codeclimate Maintainability](https://api.codeclimate.com/v1/badges/326d3c304e32645811c8/maintainability)](https://codeclimate.com/github/simonmittag/jabba/maintainability)
[![Codeclimate Test Coverage](https://api.codeclimate.com/v1/badges/326d3c304e32645811c8/test_coverage)](https://codeclimate.com/github/simonmittag/jabba/code)
[![Go Version](https://img.shields.io/github/go-mod/go-version/simonmittag/jabba)](https://img.shields.io/github/go-mod/go-version/simonmittag/jabba)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

## Features
* HTTP 1.1 compatible buffering reverse proxy w/ TLS support.
* JSON API friendly, configurable recovery of idempotent requests.
* policy based routing to multiple upstream servers for a/b testing
* highly concurrent with multiprocessor support, 10k req/s

## What's new
Jabba is heavily under development and currently alpha grade. Production
use is currently not recommended, until TLS support is fully implemented. 
Please see project milestones for details.

## Up and running

```bash
go install github.com/simonmittag/jabba/cmd/jabba && jabba
```

## Supported headers

Specify these headers to control server behaviour


### X-REQUEST-DEBUG
Set this to true to include a full trace of the request in the server log

```bash
X-REQUEST-DEBUG: true
```

## Supported environment variables

Specify these env vars to control server behaviour


### LOGLEVEL
Loglevel for stdout, one of TRACE, DEBUG, INFO or WARN

```bash
LOGLEVEL="DEBUG"
```

### TZ
TimeZone

```bash
TZ="Australia/Sydney"
```

## Contributions
The Jabba team welcomes all [contributors](https://github.com/simonmittag/jabba/blob/master/CONTRIBUTING.md). Everyone interacting with the project's codebase, issue trackers, chat rooms and mailing lists
is expected to follow the [code of conduct](https://github.com/simonmittag/jabba/blob/master/CODE_OF_CONDUCT.md)
