package main

import (
	"fmt"
	"os/exec"
	"path"
	"sync"

	"github.com/cli/go-gh"
)

type workAction int

const (
	workActionClone workAction = 0
	workActionSync  workAction = 1
)

type workCmd struct {
	action   workAction
	rootPath string
	org      string
	repo     string
}

type workers struct {
	dop      int
	ch       chan workCmd
	reporter *reporter
	logger   *logger
	wg       sync.WaitGroup
}

func newWorkers(dop int, reporter *reporter, logger *logger) *workers {
	return &workers{dop: dop, ch: make(chan workCmd), reporter: reporter, logger: logger}
}

func (w *workers) start() {
	for i := 0; i < w.dop; i++ {
		w.wg.Add(1)
		go func(index int) {
			w.logger.verboseLogf("starting sync worker %d", index)

			for cmd := range w.ch {
				switch cmd.action {
				case workActionClone:
					w.gitClone(cmd)
				case workActionSync:
					w.gitSync(cmd)
				}
			}
			w.logger.verboseLogf("sync worker %d stopped", index)
			w.wg.Done()
		}(i)
	}
}

func (w *workers) wait() {
	close(w.ch)
	w.wg.Wait()
}

func (w *workers) enqueueClone(rootPath, org, repo string) {
	w.ch <- workCmd{
		action:   workActionClone,
		rootPath: rootPath,
		org:      org,
		repo:     repo,
	}
}

func (w *workers) enqueueSync(rootPath, org, repo string) {
	w.ch <- workCmd{
		action:   workActionSync,
		rootPath: rootPath,
		org:      org,
		repo:     repo,
	}
}

func (w *workers) gitClone(cmd workCmd) {
	localPath := path.Join(cmd.rootPath, cmd.repo)
	_, bufErr, err := gh.Exec("repo", "clone", fmt.Sprintf("%s/%s", cmd.org, cmd.repo), localPath)
	if err != nil {
		w.reporter.reportCloneFailure(cmd.repo, fmt.Errorf("%s: %w", bufErr.String(), err))
		return
	}
	w.reporter.reportCloneSuccess(cmd.repo)
	w.logger.logf("%s cloned\n", cmd.repo)
}

func (w *workers) gitSync(cmd workCmd) {
	path := path.Join(cmd.rootPath, cmd.repo)
	execCmd := exec.Command("git", "-C", path, "fetch", "-p")
	err := execCmd.Run()
	if err != nil {
		w.reporter.reportSyncFailure(cmd.repo, fmt.Errorf("failed to git fetch changes: %w", err))
		return
	}
	execCmd = exec.Command("git", "-C", path, "pull")
	err = execCmd.Run()
	if err != nil {
		w.reporter.reportSyncFailure(cmd.repo, fmt.Errorf("failed to git pull changes: %w", err))
		return
	}
	w.reporter.reportSyncSuccess(cmd.repo)
	w.logger.logf("%s synced\n", cmd.repo)
}
