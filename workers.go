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
	wg       sync.WaitGroup
}

func newWorkers(dop int, reporter *reporter) *workers {
	return &workers{dop: dop, ch: make(chan workCmd), reporter: reporter}
}

func (w *workers) start() {
	for i := 0; i < w.dop; i++ {
		w.wg.Add(1)
		go func(index int) {
			w.reporter.reportWorkerStarted(index)

			for cmd := range w.ch {
				switch cmd.action {
				case workActionClone:
					w.gitClone(index, cmd)
				case workActionSync:
					w.gitSync(index, cmd)
				}
			}

			w.reporter.reportWorkerFinished(index)
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

func (w *workers) gitClone(worker int, cmd workCmd) {
	localPath := path.Join(cmd.rootPath, cmd.repo)
	_, bufErr, err := gh.Exec("repo", "clone", fmt.Sprintf("%s/%s", cmd.org, cmd.repo), localPath)
	if err != nil {
		w.reporter.reportCloneFailure(worker, cmd.repo, fmt.Errorf("%s: %w", bufErr.String(), err))
		return
	}
	w.reporter.reportCloneSuccess(worker, cmd.repo)
}

func (w *workers) gitSync(worker int, cmd workCmd) {
	path := path.Join(cmd.rootPath, cmd.repo)
	execCmd := exec.Command("git", "-C", path, "fetch", "-p", "-P")
	err := execCmd.Run()
	if err != nil {
		w.reporter.reportSyncFailure(worker, cmd.repo, fmt.Errorf("failed to git fetch changes from %s: %w", cmd.repo, err))
		return
	}
	w.reporter.reportSyncSuccess(worker, cmd.repo)
}
