package prow

import (
	"fmt"
	"io/ioutil"
	"sort"
	"time"

	gyaml "github.com/ghodss/yaml"
	"go.uber.org/zap"
	prowconfig "k8s.io/test-infra/prow/config"
)

// Jobs is a list of prow configurations.
type Jobs []Job

func (ss Jobs) Len() int      { return len(ss) }
func (ss Jobs) Swap(i, j int) { ss[i], ss[j] = ss[j], ss[i] }

// in the order of:
//  1. pre-submit, category, provider, ID, group
//  2. post-submit, category, provider, ID, group
//  3. periodic, category, provider, ID, group
func (ss Jobs) Less(i, j int) bool {
	a, b := ss[i], ss[j]
	// pre-submit should be first
	if a.Type == TypePresubmit && b.Type != TypePresubmit {
		return true
	}
	// periodic should be last
	if a.Type == TypePeriodic && b.Type != TypePeriodic {
		return false
	}
	if a.Type == TypePostsubmit && b.Type == TypePeriodic {
		return true
	}
	if a.Type == TypePostsubmit && b.Type == TypePresubmit {
		return false
	}
	// (a.Type == TypePresubmit && b.Type == TypePresubmit) ||
	// 	(a.Type == TypePostsubmit && b.Type == TypePostsubmit) ||
	// 	(a.Type == TypePeriodic && b.Type == TypePeriodic)
	if a.Category != b.Category {
		return a.Category < b.Category
	}
	if a.Provider != b.Provider {
		return a.Provider < b.Provider
	}
	if a.ID != b.ID {
		return a.ID < b.ID
	}
	return a.Group < b.Group
}

// wrap "prowconfig.Config.JobConfig"
// exclude "presets" and others
type jobConfig struct {
	Presubmits  map[string][]prowconfig.Presubmit  `json:"presubmits,omitempty"`
	Postsubmits map[string][]prowconfig.Postsubmit `json:"postsubmits,omitempty"`
	Periodics   []prowconfig.Periodic              `json:"periodics,omitempty"`
}

// LoadJobs fetches all jobs from upstream "k8s.io/test-infra".
func LoadJobs(lg *zap.Logger, paths []string) (
	jobs Jobs,
	all map[string]Job,
	categoryToProviderToJob map[string]map[string]Job,
	err error) {
	all = make(map[string]Job)
	categoryToProviderToJob = make(map[string]map[string]Job)
	for _, prowPath := range paths {
		lg.Info("parsing prow configuration", zap.String("path", prowPath))
		var d []byte
		d, err = ioutil.ReadFile(prowPath)
		if err != nil {
			return nil, nil, nil, err
		}

		var cfg jobConfig
		if err = gyaml.Unmarshal(d, &cfg); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to parse %q (%v)", prowPath, err)
		}

		base := Job{URL: getProwURL(prowPath)}

		// pre-submit
		for group, ps := range cfg.Presubmits {
			for _, v := range ps {
				cur := base
				cur.Type = TypePresubmit
				cur.Group = toGroup(group, v.Name)
				cur.Category = categorizePresubmit(v)
				cur.Provider = toProvider(group, v.Name)
				cur.ID = v.Name
				cur.Branches = copyStrings(v.Branches)
				cur.Interval = time.Duration(0)
				cur.StatusURL = getProwStatusURL(v.Name)
				munge(lg, cur, all, categoryToProviderToJob)
				for _, v2 := range v.RunAfterSuccess {
					cur := base
					cur.Type = TypePresubmit
					cur.Group = toGroup(group, v2.Name)
					cur.Category = categorizePresubmit(v2)
					cur.Provider = toProvider(group, v2.Name)
					cur.ID = v2.Name
					cur.Branches = copyStrings(v2.Branches)
					cur.Interval = time.Duration(0)
					cur.StatusURL = getProwStatusURL(v2.Name)
					munge(lg, cur, all, categoryToProviderToJob)
				}
			}
		}
		// post-submit
		for group, ps := range cfg.Postsubmits {
			for _, v := range ps {
				cur := base
				cur.Type = TypePostsubmit
				cur.Group = toGroup(group, v.Name)
				cur.Category = categorizePostsubmit(v)
				cur.Provider = toProvider(group, v.Name)
				cur.ID = v.Name
				cur.Branches = copyStrings(v.Branches)
				cur.Interval = time.Duration(0)
				cur.StatusURL = getProwStatusURL(v.Name)
				munge(lg, cur, all, categoryToProviderToJob)
				for _, v2 := range v.RunAfterSuccess {
					cur := base
					cur.Type = TypePostsubmit
					cur.Group = toGroup(group, v2.Name)
					cur.Category = categorizePostsubmit(v2)
					cur.Provider = toProvider(group, v2.Name)
					cur.ID = v2.Name
					cur.Branches = copyStrings(v2.Branches)
					cur.Interval = time.Duration(0)
					cur.StatusURL = getProwStatusURL(v2.Name)
					munge(lg, cur, all, categoryToProviderToJob)
				}
			}
		}
		// periodic
		for _, v := range cfg.Periodics {
			cur := base
			cur.Type = TypePeriodic
			cur.Group = toGroup("periodic", v.Name)
			cur.Category = categorizePeriodic(v)
			cur.Provider = toProvider("periodic", v.Name)
			cur.ID = v.Name
			cur.Branches = nil
			cur.Interval = v.GetInterval()
			cur.StatusURL = getProwStatusURL(v.Name)
			munge(lg, cur, all, categoryToProviderToJob)
			for _, v2 := range v.RunAfterSuccess {
				cur := base
				cur.Type = TypePeriodic
				cur.Group = toGroup("periodic", v.Name)
				cur.Category = categorizePeriodic(v)
				cur.Provider = toProvider("periodic", v.Name)
				cur.ID = v2.Name
				cur.Branches = nil
				cur.Interval = v2.GetInterval()
				cur.StatusURL = getProwStatusURL(v2.Name)
				munge(lg, cur, all, categoryToProviderToJob)
			}
		}
	}

	jobs = make(Jobs, 0, len(all))
	for _, cur := range all {
		jobs = append(jobs, cur)
		lg.Debug("adding a job",
			zap.String("type", cur.Type),
			zap.String("group", cur.Group),
			zap.String("category", cur.Category),
			zap.String("provider", cur.Provider),
			zap.String("id", cur.ID),
		)
	}
	sort.Sort(Jobs(jobs))
	lg.Info("fetched all jobs from 'k8s.io/test-infra'",
		zap.Int("jobs", len(jobs)),
		zap.Int("categories", len(categoryToProviderToJob)),
	)
	return jobs, all, categoryToProviderToJob, err
}

func munge(
	lg *zap.Logger,
	job Job,
	all map[string]Job,
	categoryToProviderToJob map[string]map[string]Job,
) {
	prev, ok := all[job.ID]
	if !ok {
		all[job.ID] = job
		if _, ok = categoryToProviderToJob[job.Category]; !ok {
			categoryToProviderToJob[job.Category] = make(map[string]Job)
			categoryToProviderToJob[job.Category][job.Provider] = job
		} else if prev, ok = categoryToProviderToJob[job.Category][job.Provider]; ok {
			// e.g. "pull-kubernetes-e2e-gke" has duplicate entries to test different branches
			lg.Warn("merging duplicate category", zap.String("id", job.ID))
			prev.Branches = mergeStrings(prev.Branches, job.Branches)
			categoryToProviderToJob[prev.Category][prev.Provider] = prev
		}
		return
	}
	// e.g. "pull-kubernetes-e2e-gce" has duplicate entries to test different branches
	prev.Branches = mergeStrings(prev.Branches, job.Branches)
	all[job.ID] = prev
	categoryToProviderToJob[job.Category][job.Provider] = prev
	lg.Warn("merging duplicate job", zap.String("id", job.ID))
}
