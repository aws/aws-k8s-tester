package prow

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"
)

// Git represents the git information.
type Git struct {
	// URL is the URL of the git repository.
	// (e.g. "https://github.com/kubernetes/kubernetes" for "k8s.io/kubernetes/kubernetes").
	URL string
	// Name is the name of this repository
	// (e.g. "test-infra" for "k8s.io/kubernetes/test-infra").
	Name string
	// Branch is the name of the branch when and where API was requested.
	Branch string
	// CommitSHA is the latest git commit SHA when API was requested.
	CommitSHA string
	// CommitURL is the URL of the original git commit.
	CommitURL string
	// CommitTimeUTC is the time of commit in UTC timezone.
	CommitTimeUTC time.Time
	// CommitTimeSeattle is the time of commit in Seattle timezone.
	CommitTimeSeattle time.Time
}

// FetchGit fetches git information from remote git server.
func FetchGit(remoteGit, branch string) (git Git, err error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	git = Git{URL: remoteGit, Name: path.Base(remoteGit), Branch: branch}
	switch {
	case strings.HasPrefix(remoteGit, "https://github.com/"):
		// e.g. "https://api.github.com/repos/kubernetes/kubernetes/git/refs/heads/master"
		resp, err := http.Get(fmt.Sprintf(
			"https://api.github.com/repos/kubernetes/%s/git/refs/heads/%s",
			git.Name,
			git.Branch,
		))
		if err != nil {
			return Git{}, err
		}
		var gr ghGitRef
		if err = json.NewDecoder(resp.Body).Decode(&gr); err != nil {
			return Git{}, err
		}
		resp.Body.Close()
		git.CommitSHA = gr.Object.SHA

		// e.g. "https://api.github.com/repos/kubernetes/kubernetes/git/commits/c98aa0a5c1e897a2eaa9f2ed7a5a980114ceedfc"
		resp, err = http.Get(fmt.Sprintf(
			"https://api.github.com/repos/kubernetes/%s/git/commits/%s",
			git.Name,
			git.CommitSHA,
		))
		if err != nil {
			return Git{}, err
		}
		var gc ghGitCommit
		if err = json.NewDecoder(resp.Body).Decode(&gc); err != nil {
			return Git{}, err
		}
		resp.Body.Close()
		git.CommitURL = gc.HTMLURL
		git.CommitTimeUTC, err = time.Parse(time.RFC3339, gc.Committer.Date)
		if err != nil {
			return Git{}, err
		}
		var loc *time.Location
		loc, err = time.LoadLocation("America/Los_Angeles")
		if err != nil {
			return Git{}, err
		}
		git.CommitTimeSeattle = git.CommitTimeUTC.In(loc)

	default:
		return Git{}, fmt.Errorf("unknown remote git server %q", remoteGit)
	}

	return git, nil
}

type ghGitRef struct {
	Object struct {
		SHA string `json:"sha"`
	} `json:"object"`
}

type ghGitCommit struct {
	HTMLURL   string `json:"html_url"`
	Committer struct {
		Date string `json:"date"`
	} `json:"committer"`
}
