# gh-orgsync

A GitHub (`gh`) CLI extension to sync all repos from a GitHub organization. Leverages Go concurrency to speed things up.
The extension will get a list of repos in the org and a list of the repos in the target local path and create an action plan.

* Repos that do not exist locally will be cloned
* Repos that exist locally and in the org will be updated (`git fetch`)
* Repos that exist locally but not in the org will not be touched, but can be reported

## Installation

1. Install the `gh` CLI - see the [installation](https://github.com/cli/cli#installation)

   _Installation requires a minimum version (2.0.0) of the the GitHub CLI that supports extensions._

2. Install this extension:

   ```sh
   gh extension install mantzas/gh-orgsync
   ```

## Flags

The extension contains multiple required and optional flags.

* Org `--org {{orgname}}` which is the org we want to sync. It is the only required flag
* Path `--path {{path to local folder}} which defines the folder to sync to

flag.StringVar(&cfg.path, "path", "", "local path for syncing repos")
 flag.StringVar(&cfg.org, "org", "", "org to be synced")
 flag.IntVar(&cfg.dop, "dop", 50, "degree of parallelism for actions")
 flag.BoolVar(&cfg.dryRun, "dry-run", false, "enable dry run")
 flag.StringVar(&cfg.reportFields, "report", "error", "comma separated list of the following values to report: error, cloned, synced and other. defaults to error")
 flag.BoolVar(&cfg.verbose, "verbose", false, "enable verbose logging")
 flag.Parse()

## Usage

Run:

```sh
gh orgsync --org {{add your GH org here}}
```
