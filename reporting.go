package main

import "fmt"

type action int

const (
	cloneAction        action = 0
	syncAction         action = 1
	workerStartAction  action = 2
	workerFinishAction action = 3
)

type reportingCmd struct {
	worker   int
	action   action
	repoName string
	err      error
}

type cloneStat struct {
	total     int
	succeeded int
	failed    int
}

type cloneResult struct {
	names  []string
	errors []error
}

type syncStat struct {
	total     int
	succeeded int
	failed    int
}

type syncResult struct {
	names  []string
	errors []error
}

type otherStat struct {
	total int
}

type workerStats struct {
	started  int
	finished int
}

type reportingStats struct {
	cloneStat   cloneStat
	cloneResult cloneResult
	syncStat    syncStat
	syncResult  syncResult
	otherStat   otherStat
	workerStats workerStats
}

func stringSliceToMap(values []string) map[string]struct{} {
	m := make(map[string]struct{}, len(values))

	for i := 0; i < len(values); i++ {
		m[values[i]] = struct{}{}
	}

	return m
}

type reporter struct {
	verbose         bool
	chReporting     chan reportingCmd
	chReportingDone chan struct{}
}

func newReporter(verbose bool) *reporter {
	return &reporter{chReporting: make(chan reportingCmd, 1000), chReportingDone: make(chan struct{}), verbose: verbose}
}

func (r *reporter) process(clone, sync, others int) {
	stats := reportingStats{
		cloneStat: cloneStat{
			total: clone,
		},
		syncStat: syncStat{
			total: sync,
		},
		otherStat: otherStat{
			total: others,
		},
	}

	for cmd := range r.chReporting {
		switch cmd.action {
		case cloneAction:
			if cmd.err != nil {
				stats.cloneStat.failed++
				stats.cloneResult.errors = append(stats.cloneResult.errors, cmd.err)
			} else {
				stats.cloneStat.succeeded++
				stats.cloneResult.names = append(stats.cloneResult.names, cmd.repoName)
			}
		case syncAction:
			if cmd.err != nil {
				stats.syncStat.failed++
				stats.syncResult.errors = append(stats.syncResult.errors, cmd.err)
			} else {
				stats.syncStat.succeeded++
				stats.syncResult.names = append(stats.syncResult.names, cmd.repoName)
			}
		case workerStartAction:
			stats.workerStats.started++
			if r.verbose {
				fmt.Printf("worker %d started\n", cmd.worker)
			}
		case workerFinishAction:
			stats.workerStats.finished++
			if r.verbose {
				fmt.Printf("worker %d finished\n", cmd.worker)
			}
		}
	}
	// TODO: report synce, cloned errors etc.

	fmt.Printf("stats: workers: %+v, clone: %+v, sync: %+v, other: %+v\n", stats.workerStats, stats.cloneStat,
		stats.syncStat, stats.otherStat)
	r.chReportingDone <- struct{}{}
}

func (r *reporter) reportSyncSuccess(worker int, repo string) {
	r.chReporting <- reportingCmd{action: syncAction, repoName: repo, worker: worker}
}

func (r *reporter) reportSyncFailure(worker int, repo string, err error) {
	r.chReporting <- reportingCmd{action: syncAction, repoName: repo, err: err, worker: worker}
}

func (r *reporter) reportCloneSuccess(worker int, repo string) {
	r.chReporting <- reportingCmd{action: cloneAction, repoName: repo, worker: worker}
}

func (r *reporter) reportCloneFailure(worker int, repo string, err error) {
	r.chReporting <- reportingCmd{action: cloneAction, repoName: repo, err: err, worker: worker}
}

func (r *reporter) reportWorkerStarted(worker int) {
	r.chReporting <- reportingCmd{action: workerStartAction, worker: worker}
}

func (r *reporter) reportWorkerFinished(worker int) {
	r.chReporting <- reportingCmd{action: workerFinishAction, worker: worker}
}

func (r *reporter) wait() {
	close(r.chReporting)
	<-r.chReportingDone
}

func (r *reporter) dryRun(clone []string, sync []string, other []string) {
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
