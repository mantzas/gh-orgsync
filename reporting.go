package main

import "fmt"

type action int

const (
	cloneAction action = 0
	syncAction  action = 1
)

type reportingCmd struct {
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

type reportingStats struct {
	cloneStat   cloneStat
	cloneResult cloneResult
	syncStat    syncStat
	syncResult  syncResult
	otherStat   otherStat
}

func stringSliceToMap(values []string) map[string]struct{} {
	m := make(map[string]struct{}, len(values))

	for i := 0; i < len(values); i++ {
		m[values[i]] = struct{}{}
	}

	return m
}

type reporter struct {
	chReporting     chan reportingCmd
	chReportingDone chan struct{}
	logger          *logger
}

func newReporter(logger *logger, verbose bool) *reporter {
	return &reporter{chReporting: make(chan reportingCmd, 1000), chReportingDone: make(chan struct{}), logger: logger}
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
		}
	}

	r.logger.logf("stats: clone: %+v, sync: %+v, other: %+v\n", stats.cloneStat, stats.syncStat, stats.otherStat)
	r.chReportingDone <- struct{}{}
}

func (r *reporter) reportSyncSuccess(repoName string) {
	r.chReporting <- reportingCmd{action: syncAction, repoName: repoName}
}

func (r *reporter) reportSyncFailure(repo string, err error) {
	r.chReporting <- reportingCmd{action: syncAction, repoName: repo, err: err}
}

func (r *reporter) reportCloneSuccess(repo string) {
	r.chReporting <- reportingCmd{action: cloneAction, repoName: repo}
}

func (r *reporter) reportCloneFailure(repo string, err error) {
	r.chReporting <- reportingCmd{action: cloneAction, repoName: repo, err: err}
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
