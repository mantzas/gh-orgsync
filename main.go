package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"

	"github.com/cli/go-gh"
)

type config struct {
	org          string
	path         string
	dop          int
	reportFields string
	dryRun       bool
	verbose      bool
}

func main() {
	cfg, err := processFlags()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if isGitDirectory(cfg.path, "") {
		fmt.Println("should not be run inside a git repo")
		os.Exit(1)
	}

	reporter, err := newReporter(cfg.verbose, cfg.reportFields)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	localRepos, err := getLocalRepos(cfg.verbose, cfg.path)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	reposToSync, err := getOrgRepos(cfg.verbose, cfg.org)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	cloning, syncing, other := calculateRepoActions(cfg.verbose, cfg.org, localRepos, reposToSync)

	if cfg.dryRun {
		reporter.dryRun(cloning, syncing, other)
		return
	}

	go reporter.process(len(cloning), len(syncing), other)

	workers := newWorkers(cfg.dop, reporter)
	workers.start()

	for _, repo := range cloning {
		workers.enqueueClone(cfg.path, cfg.org, repo)
	}

	for _, repo := range syncing {
		workers.enqueueSync(cfg.path, cfg.org, repo)
	}

	workers.wait()
	reporter.wait()
}

func processFlags() (config, error) {
	cfg := config{}

	flag.StringVar(&cfg.org, "org", "", "the org we want to sync. It is the only required flag.")
	flag.StringVar(&cfg.path, "path", "", "defines the folder to sync to. When omitted local path is assumed.")
	flag.IntVar(&cfg.dop, "dop", 50, "degree of parallelism defines the number of workers which will be used. Default value is 50.")
	flag.BoolVar(&cfg.dryRun, "dry-run", false, "enable dry run")
	flag.StringVar(&cfg.reportFields, "report", "error", "which allows reporting options (error, cloned, synced and other). default value is error.")
	flag.BoolVar(&cfg.verbose, "verbose", false, "enable verbose logging")
	flag.Parse()

	if cfg.org == "" {
		flag.PrintDefaults()
		return config{}, errors.New("org was not provided")
	}

	if cfg.reportFields == "" {
		flag.PrintDefaults()
		return config{}, errors.New("report fields are not provided")
	}

	if cfg.path == "" {
		cfg.path = "."
	}

	if cfg.verbose {
		fmt.Printf("flags: %+v\n", cfg)
	}

	return cfg, nil
}

func calculateRepoActions(verbose bool, org string, localRepos, remoteRepos []string) (clone []string, sync []string, other []string) {
	remoteMap := stringSliceToMap(remoteRepos)
	localMap := stringSliceToMap(localRepos)

	// find out which repos need to be synced
	for _, localRepo := range localRepos {
		_, ok := remoteMap[localRepo]
		if !ok {
			continue
		}
		sync = append(sync, localRepo)
		delete(remoteMap, localRepo)
		delete(localMap, localRepo)
	}

	// remote maps contains only new repos to be cloned
	for k := range remoteMap {
		clone = append(clone, k)
	}

	sort.Strings(clone)

	for k := range localMap {
		other = append(other, k)
	}

	sort.Strings(other)

	if verbose {
		fmt.Printf("%d to be cloned, %d to be synced and %d other\n", len(clone), len(sync), len(other))
	}
	return
}

func getOrgRepos(verbose bool, org string) ([]string, error) {
	bufOut, bufErr, err := gh.Exec("repo", "list", org, "-L", "5000", "--json", "name")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", bufErr.String(), err)
	}

	var reposToSync []struct {
		Name string `json:"name"`
	}

	err = json.Unmarshal(bufOut.Bytes(), &reposToSync)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON response from GH: %w", err)
	}

	var repos []string

	for _, repo := range reposToSync {
		repos = append(repos, repo.Name)
	}

	if verbose {
		fmt.Printf("found %d remote repos\n", len(repos))
	}

	return repos, nil
}

func getLocalRepos(verbose bool, path string) ([]string, error) {
	var folders []string

	rootPath := filepath.Clean(path)

	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if !isGitDirectory(rootPath, entry.Name()) {
			continue
		}

		folders = append(folders, entry.Name())
	}

	if verbose {
		fmt.Printf("found %d local repos\n", len(folders))
	}

	return folders, nil
}

func isGitDirectory(rootPath, directoryName string) bool {
	path := path.Join(rootPath, directoryName)
	cmd := exec.Command("git", "-C", path, "rev-parse")
	err := cmd.Run()
	return err == nil
}
