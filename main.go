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
	"sync"

	"github.com/cli/go-gh"
)

type config struct {
	org           string
	path          string
	repoListLimit string
	dop           int
	dryRun        bool
	verbose       bool
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

	localRepos, err := getLocalRepos(cfg.path)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	reposToSync, err := getOrgRepos(cfg.org, cfg.repoListLimit)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	clone, sync, other := calculateRepoActions(cfg.org, localRepos, reposToSync)

	if cfg.dryRun {
		reportDryRun(clone, sync, other)
		return
	}

	cns := &console{}

	cloneRepos(cns, cfg.dop, cfg.path, cfg.org, clone)
	syncRepos(cns, cfg.dop, cfg.path, sync)

	fmt.Printf("%d cloned, %d synced. %d repos exist locally but not in GH", 0, 0, len(other))
}

func processFlags() (config, error) {
	cfg := config{}

	flag.StringVar(&cfg.path, "path", "", "local path for syncing repos")
	flag.StringVar(&cfg.org, "org", "", "org to be synced")
	flag.StringVar(&cfg.repoListLimit, "repo-list-limit", "5000", "repo list limit setting")
	flag.IntVar(&cfg.dop, "dop", 20, "degree of parallelism for actions")
	flag.BoolVar(&cfg.dryRun, "dry-run", false, "enable dry run")
	flag.BoolVar(&cfg.verbose, "verbose", false, "enable verbose logging")
	flag.Parse()

	if cfg.org == "" {
		flag.PrintDefaults()
		return config{}, errors.New("org was not provided")
	}

	if cfg.path == "" {
		cfg.path = "."
	}

	fmt.Printf("flags: %+v\n", cfg)
	return cfg, nil
}

func cloneRepos(cns *console, dop int, rootPath, org string, repos []string) {
	if len(repos) == 0 {
		cns.Println("nothing to clone")
		return
	}
	ch := make(chan cloneCmd)

	cns.Printf("starting %d clone workers\n", dop)

	for i := 0; i < dop; i++ {
		go func(count int, cns *console) {
			cns.Printf("starting clone worker %d\n", count)
			for cmd := range ch {
				gitClone(cns, cmd)
			}
		}(i, cns)
	}

	for _, repo := range repos {
		ch <- cloneCmd{rootPath: rootPath, org: org, repo: repo}
	}

	close(ch)
}

func syncRepos(cns *console, dop int, rootPath string, names []string) {
	if len(names) == 0 {
		cns.Println("nothing to sync")
		return
	}
	ch := make(chan syncCmd)

	cns.Printf("starting %d sync workers\n", dop)

	for i := 0; i < dop; i++ {
		go func(count int, cns *console) {
			cns.Printf("starting sync worker %d\n", count)
			for cmd := range ch {
				gitSync(cns, cmd)
			}
		}(i, cns)
	}

	for _, name := range names {
		ch <- syncCmd{rootPath: rootPath, directoryName: name}
	}

	close(ch)
}

func calculateRepoActions(org string, localRepos, remoteRepos []string) (clone []string, sync []string, other []string) {
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

	fmt.Printf("%d to be cloned, %d to be synced and %d other\n", len(clone), len(sync), len(other))
	return
}

func getOrgRepos(org, repoListLimit string) ([]string, error) {
	bufOut, bufErr, err := gh.Exec("repo", "list", org, "-L", repoListLimit, "--json", "name")
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

	fmt.Printf("found %d remote repos\n", len(repos))
	return repos, nil
}

func getLocalRepos(path string) ([]string, error) {
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

	fmt.Printf("found %d local repos\n", len(folders))
	return folders, nil
}

func isGitDirectory(rootPath, directoryName string) bool {
	path := path.Join(rootPath, directoryName)
	cmd := exec.Command("git", "-C", path, "rev-parse")
	err := cmd.Run()
	return err == nil
}

type syncCmd struct {
	rootPath      string
	directoryName string
}

func gitSync(cns *console, cmd syncCmd) {
	cns.Printf("syncing %s\n", cmd.directoryName)
	path := path.Join(cmd.rootPath, cmd.directoryName)
	execCmd := exec.Command("git", "-C", path, "fetch", "-p")
	err := execCmd.Run()
	if err != nil {
		cns.Printf("failed to git fetch changes: %v\n", err)
		return
	}
	execCmd = exec.Command("git", "-C", path, "pull")
	err = execCmd.Run()
	if err != nil {
		cns.Printf("failed to git pull changes in %s: %v\n", cmd.directoryName, err)
	}
}

type cloneCmd struct {
	rootPath string
	org      string
	repo     string
}

func gitClone(cns *console, cmd cloneCmd) {
	cns.Printf("cloning %s\n", cmd.repo)
	localPath := path.Join(cmd.rootPath, cmd.repo)
	_, bufErr, err := gh.Exec("repo", "clone", repoWithOwner(cmd.org, cmd.repo), localPath)
	cns.Printf("%s: %v", bufErr.String(), err)
}

func repoWithOwner(org, name string) string {
	return fmt.Sprintf("%s/%s", org, name)
}

func reportDryRun(clone []string, sync []string, other []string) {
	printSection("Clone", clone)
	printSection("Sync", sync)
	printSection("Other", other)
}

func printSection(section string, values []string) {
	fmt.Println(section)
	if len(values) == 0 {
		fmt.Println("nothing to do")
		return
	}

	for i, value := range values {
		fmt.Printf("%5d %s\n", i+1, value)
	}
}

func stringSliceToMap(values []string) map[string]struct{} {
	m := make(map[string]struct{}, len(values))

	for i := 0; i < len(values); i++ {
		m[values[i]] = struct{}{}
	}

	return m
}

type console struct {
	mu sync.Mutex
}

func (c *console) Printf(format string, a ...any) {
	c.mu.Lock()
	fmt.Printf(format, a...)
	c.mu.Unlock()
}

func (c *console) Println(a ...any) {
	c.mu.Lock()
	fmt.Println(a...)
	c.mu.Unlock()
}
