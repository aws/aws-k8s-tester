package status

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/aws/awstester/internal/prow"

	"go.uber.org/zap"
)

type status struct {
	lg *zap.Logger

	mu                      sync.RWMutex
	gitK8s                  prow.Git
	gitTestInfra            prow.Git
	jobs                    prow.Jobs
	all                     map[string]prow.Job
	categoryToProviderToJob map[string]map[string]prow.Job

	statusMu            sync.RWMutex
	statusUpdateLimit   time.Duration
	statusUpdated       time.Time
	statusHTMLHead      string
	statusHTMLUpdateMsg string
	statusHTMLGitRows   string
	statusHTMLJobRows   string
	statusHTMLEnd       string
}

func newStatus(lg *zap.Logger) *status {
	return &status{
		lg: lg,

		statusUpdated:       time.Now().UTC().Add(-3 * time.Hour),
		statusUpdateLimit:   3 * time.Hour, // max 1 request for every 3-hour
		statusHTMLHead:      htmlHead,
		statusHTMLUpdateMsg: "",
		statusHTMLGitRows:   "",
		statusHTMLJobRows:   "",
		statusHTMLEnd:       upstreamHTMLEnd,
	}
}

func (s *status) getSummary() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	n := int64(len(s.all))
	awsN, gcpN, notCategorizedN := int64(0), int64(0), int64(0)
	for _, v := range s.all {
		switch v.Provider {
		case prow.ProviderAWS:
			awsN++
		case prow.ProviderGCP:
			gcpN++
		case prow.ProviderNotCategorized:
			notCategorizedN++
		}
	}
	awsPct := (float64(awsN) / float64(n)) * 100
	gcpPct := (float64(gcpN) / float64(n)) * 100
	notCategorizedPct := (float64(notCategorizedN) / float64(n)) * 100

	return createUpdateMsgSummary(n, awsN, gcpN, notCategorizedN, awsPct, gcpPct, notCategorizedPct)
}

func (s *status) refresh() {
	s.mu.Lock()
	s.lg.Info("refreshing")
	defer func() {
		s.mu.Unlock()
		s.lg.Info("refreshed")
	}()

	n := int64(len(s.all))
	awsN, gcpN, notCategorizedN := int64(0), int64(0), int64(0)
	for _, v := range s.all {
		switch v.Provider {
		case prow.ProviderAWS:
			awsN++
		case prow.ProviderGCP:
			gcpN++
		case prow.ProviderNotCategorized:
			notCategorizedN++
		}
	}
	awsPct := (float64(awsN) / float64(n)) * 100
	gcpPct := (float64(gcpN) / float64(n)) * 100
	notCategorizedPct := (float64(notCategorizedN) / float64(n)) * 100

	s.statusMu.RLock()
	took := time.Now().UTC().Sub(s.statusUpdated)
	ok := took < s.statusUpdateLimit
	s.statusMu.RUnlock()
	if ok {
		s.statusMu.Lock()
		s.statusHTMLUpdateMsg = createUpdateMsg(n, awsN, gcpN, notCategorizedN, awsPct, gcpPct, notCategorizedPct, s.statusUpdated, fmt.Errorf("rate limit exceeded %v", s.statusUpdateLimit))
		s.statusMu.Unlock()
		return
	}

	var err error
	s.gitK8s, err = prow.FetchGit("https://github.com/kubernetes/kubernetes", "master")
	if err != nil {
		s.lg.Warn("failed to fetch 'kubernetes' git", zap.Error(err))
		s.statusMu.Lock()
		s.statusHTMLUpdateMsg = createUpdateMsg(n, awsN, gcpN, notCategorizedN, awsPct, gcpPct, notCategorizedPct, s.statusUpdated, err)
		s.statusMu.Unlock()
		return
	}
	s.gitTestInfra, err = prow.FetchGit("https://github.com/kubernetes/test-infra", "master")
	if err != nil {
		s.lg.Warn("failed to fetch 'test-infra' git", zap.Error(err))
		s.statusMu.Lock()
		s.statusHTMLUpdateMsg = createUpdateMsg(n, awsN, gcpN, notCategorizedN, awsPct, gcpPct, notCategorizedPct, s.statusUpdated, err)
		s.statusMu.Unlock()
		return
	}

	var dir string
	var paths []string
	dir, paths, err = prow.DownloadJobsUpstream(s.lg)
	if err != nil {
		s.lg.Warn("failed to download 'test-infra' git", zap.Error(err))
		s.statusMu.Lock()
		s.statusHTMLUpdateMsg = createUpdateMsg(n, awsN, gcpN, notCategorizedN, awsPct, gcpPct, notCategorizedPct, s.statusUpdated, err)
		s.statusMu.Unlock()
		return
	}
	defer os.RemoveAll(dir)

	s.jobs, s.all, s.categoryToProviderToJob, err = prow.LoadJobs(s.lg, paths)
	if err != nil {
		s.lg.Warn("failed to fetch jobs", zap.Error(err))
		s.statusMu.Lock()
		s.statusHTMLUpdateMsg = createUpdateMsg(n, awsN, gcpN, notCategorizedN, awsPct, gcpPct, notCategorizedPct, s.statusUpdated, err)
		s.statusMu.Unlock()
		return
	}

	n = int64(len(s.all))
	awsN, gcpN, notCategorizedN = int64(0), int64(0), int64(0)
	for _, v := range s.all {
		switch v.Provider {
		case prow.ProviderAWS:
			awsN++
		case prow.ProviderGCP:
			gcpN++
		case prow.ProviderNotCategorized:
			notCategorizedN++
		}
	}
	awsPct = (float64(awsN) / float64(n)) * 100
	gcpPct = (float64(gcpN) / float64(n)) * 100
	notCategorizedPct = (float64(notCategorizedN) / float64(n)) * 100

	now := time.Now().UTC()
	s.statusMu.Lock()
	s.statusUpdated = now
	s.statusHTMLHead = htmlHead
	s.statusHTMLUpdateMsg = createUpdateMsg(n, awsN, gcpN, notCategorizedN, awsPct, gcpPct, notCategorizedPct, s.statusUpdated, nil)
	s.statusHTMLGitRows = createGitRows(now, []prow.Git{s.gitK8s, s.gitTestInfra})
	s.statusHTMLJobRows = createJobRows(s.jobs, s.all, s.categoryToProviderToJob)
	s.statusHTMLEnd = upstreamHTMLEnd
	s.statusMu.Unlock()
}
