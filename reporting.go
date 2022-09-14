package main

import (
	"errors"
	"fmt"
	"strings"
)

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
	names []string
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
	fields          []string
	chReporting     chan reportingCmd
	chReportingDone chan struct{}
}

func newReporter(verbose bool, reportFields string) (*reporter, error) {
	fields, err := validateReportFields(reportFields)
	if err != nil {
		return nil, err
	}

	return &reporter{
		chReporting: make(chan reportingCmd, 1000), chReportingDone: make(chan struct{}),
		verbose: verbose, fields: fields,
	}, nil
}

func validateReportFields(value string) ([]string, error) {
	if value == "" {
		return nil, errors.New("fields are empty")
	}

	fieldMap := map[string]int{
		"error":  0,
		"cloned": 0,
		"synced": 0,
		"other":  0,
		"all":    0,
	}

	fields := strings.Split(value, ",")

	for _, field := range fields {
		_, ok := fieldMap[field]
		if !ok {
			return nil, fmt.Errorf("report field %s is invalid", field)
		}
		fieldMap[field]++
	}

	for field, count := range fieldMap {
		if count > 1 {
			return nil, fmt.Errorf("report field %s exists multiple times", field)
		}
	}

	return fields, nil
}

func (r *reporter) process(clone, sync int, others []string) {
	stats := reportingStats{
		cloneStat: cloneStat{
			total: clone,
		},
		syncStat: syncStat{
			total: sync,
		},
		otherStat: otherStat{
			names: others,
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

	if r.verbose {
		fmt.Printf("stats: workers: %+v, clone: %+v, sync: %+v, other: %d\n", stats.workerStats, stats.cloneStat,
			stats.syncStat, len(stats.otherStat.names))
	}

	r.print(stats)

	r.chReportingDone <- struct{}{}
}

func (r *reporter) print(stats reportingStats) {
	for _, field := range r.fields {
		switch field {
		case "error":
			r.printErrors(stats.cloneResult.errors, stats.syncResult.errors)
		case "cloned":
			printSection("Cloned", stats.cloneResult.names)
		case "synced":
			printSection("Synced", stats.cloneResult.names)
		case "other":
			printSection("Other", stats.cloneResult.names)
		}
	}
}

func (r *reporter) printErrors(cloneErrors, syncErrors []error) {
	if len(cloneErrors) != 0 {
		fmt.Println("Clone failures")
		for _, err := range cloneErrors {
			fmt.Println(err)
		}
	}

	if len(syncErrors) == 0 {
		return
	}

	fmt.Println("Sync failures")
	for _, err := range syncErrors {
		fmt.Println(err)
	}
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
		fmt.Println("n/a")
		return
	}

	for i, value := range values {
		fmt.Printf("%5d %s\n", i+1, value)
	}
}
