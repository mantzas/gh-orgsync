package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/cli/go-gh"
)

func main() {
	var org string
	var path string
	var repoListLimit string
	var dryRun bool
	var verbose bool

	flag.StringVar(&path, "path", "", "local path for syncing repos")
	flag.StringVar(&org, "org", "", "org to be synced")
	flag.StringVar(&repoListLimit, "repo-list-limit", "5000", "repo list limit setting")
	flag.BoolVar(&dryRun, "dry-run", false, "enable dry run")
	flag.BoolVar(&verbose, "verbose", false, "enable verbose logging")
	flag.Parse()

	if org == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	if path == "" {
		path = "."
	}

	fmt.Printf("running with org: %s in path %s with dry-run %s\n", org, path, strconv.FormatBool(dryRun))

	if isGitDirectory(path, "") {
		fmt.Println("should not be run inside a git repo")
		os.Exit(1)
	}

	localRepos, err := getLocalRepos(path)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	reposToSync, err := getOrgRepos(org, repoListLimit)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	clone, sync, other := calculateRepoActions(org, localRepos, reposToSync)

	if dryRun {
		reportDryRun(clone, sync, other)
		return
	}

	cloned, err := cloneRepos(path, org, clone)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	synced, err := syncRepos(path, sync)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Printf("%d cloned, %d synced. %d repos exist locally but not in GH", cloned, synced, len(other))
}

func cloneRepos(rootPath, org string, repos []string) (int, error) {
	cloned := 0
	for _, repo := range repos {
		err := gitClone(rootPath, org, repo)
		if err != nil {
			return cloned, err
		}
		cloned++
	}
	return cloned, nil
}

func syncRepos(rootPath string, names []string) (int, error) {
	synced := 0
	for _, name := range names {
		err := gitSync(rootPath, name)
		if err != nil {
			return synced, err
		}
		synced++
	}
	return synced, nil
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

func gitSync(rootPath, directoryName string) error {
	fmt.Printf("syncing %s\n", directoryName)
	path := path.Join(rootPath, directoryName)
	cmd := exec.Command("git", "-C", path, "fetch", "-p")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to git fetch changes: %w", err)
	}
	cmd = exec.Command("git", "-C", path, "pull")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to git pull changes in %s: %w", directoryName, err)
	}
	return nil
}

func gitClone(rootPath, org, name string) error {
	fmt.Printf("cloning %s\n", name)
	localPath := path.Join(rootPath, name)
	_, bufErr, err := gh.Exec("repo", "clone", repoWithOwner(org, name), localPath)
	if err != nil {
		return fmt.Errorf("%s: %w", bufErr.String(), err)
	}

	return nil
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
