package status

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"sort"
	"time"

	"github.com/aws/awstester/internal/prow"

	"github.com/dustin/go-humanize"
)

const htmlHead = `<!DOCTYPE html>
<html>
<head>
<style>
table {
	font-family: arial, sans-serif;
	border-collapse: collapse;
	width: 100%;
}

td, th {
	border: 1px solid #dddddd;
	text-align: center;
}
</style>
</head>
`

const tmplUpdateMsg = `
<body>
<h2>EKS Testing Status: Upstream</h2>

<br>
<b>Total:</b> {{.TestsN}} tests<br>
<b>AWS:</b> {{.TestsAWS}} tests ({{.TestsAWSPct}})<br>
<b>GCP:</b> {{.TestsGCP}} tests ({{.TestsGCPPct}})<br>
<b>Not-Categorized:</b> {{.TestsNotCategorized}} tests ({{.TestsNotCategorizedPct}})<br>
<b>Message:</b> {{.UpdateMsg}}<br>
<br>
<br>

`

type updateMsg struct {
	TestsN                 string
	TestsAWS               string
	TestsAWSPct            string
	TestsGCP               string
	TestsGCPPct            string
	TestsNotCategorized    string
	TestsNotCategorizedPct string
	UpdateMsg              string
}

func createUpdateMsg(n, awsN, gcpN, notCategorizedN int64, awsPct, gcpPct, notCategorizedPct float64, lastUpdate time.Time, err error) string {
	tmpl := template.Must(template.New("tmplUpdateMsg").Parse(tmplUpdateMsg))
	msg := updateMsg{
		TestsN:                 humanize.Comma(n),
		TestsAWS:               humanize.Comma(awsN),
		TestsAWSPct:            fmt.Sprintf("%.3f %%", awsPct),
		TestsGCP:               humanize.Comma(gcpN),
		TestsGCPPct:            fmt.Sprintf("%.3f %%", gcpPct),
		TestsNotCategorized:    humanize.Comma(notCategorizedN),
		TestsNotCategorizedPct: fmt.Sprintf("%.3f %%", notCategorizedPct),
		UpdateMsg:              fmt.Sprintf("last update was %s", humanize.Time(lastUpdate)),
	}
	if err != nil {
		msg.UpdateMsg += fmt.Sprintf(" (%v)", err)
	}
	buf := bytes.NewBuffer(nil)
	if err = tmpl.Execute(buf, &msg); err != nil {
		panic(err)
	}
	return html.UnescapeString(buf.String())
}

const tmplGitRows = `
<table>
	<tr>
		<th>Name</th>
		<th>Branch</th>
		<th>Commit</th>
		<th>Time (Seattle, WA)</th>
	</tr>
{{range .}}
	<tr>
		<td>{{.Name}}</td>
		<td>{{.Branch}}</td>
		<td>{{.Commit}}</td>
		<td>{{.TimeSeattleWA}}</td>
	</tr>
{{end}}
</table>
<br>
`

type gitRow struct {
	Name          string
	Branch        string
	Commit        string
	TimeSeattleWA string
}

func createGitRows(now time.Time, gs []prow.Git) string {
	rows := make([]gitRow, len(gs))
	for i, git := range gs {
		rows[i] = gitRow{
			Name:   fmt.Sprintf(`<a href="%s" target="_blank">%s</a>`, git.URL, git.Name),
			Branch: git.Branch,
			Commit: fmt.Sprintf(`<a href="%s" target="_blank">%s</a>`, git.CommitURL, git.CommitSHA[:9]),
			TimeSeattleWA: fmt.Sprintf("%s (%s)",
				git.CommitTimeSeattle.String(),
				humanize.RelTime(git.CommitTimeSeattle, now, "ago", "from now"),
			),
		}
	}
	tmpl := template.Must(template.New("tmplGitRows").Parse(tmplGitRows))
	buf := bytes.NewBuffer(nil)
	if err := tmpl.Execute(buf, rows); err != nil {
		panic(err)
	}
	return html.UnescapeString(buf.String())
}

const tmplJobRows = `
<table>
	<tr>
		<th>Type</th>
		<th>Group</th>
		<th>Category</th>
		<th>Provider AWS</th>
		<th>Provider GCP</th>
		<th>Provider Not-Categorized</th>
	</tr>
{{range .}}
	<tr>
		<td>{{.Type}}</td>
		<td>{{.Group}}</td>
		<td>{{.Category}}</td>
		<td>{{.ProviderAWS}}</td>
		<td>{{.ProviderGCP}}</td>
		<td>{{.ProviderNotCategorized}}</td>
	</tr>
{{end}}
</table>
`

type jobRow struct {
	Type                   string
	Group                  string
	Category               string
	ProviderAWS            string
	ProviderGCP            string
	ProviderNotCategorized string
}

type jobRows []jobRow

func (ss jobRows) Len() int      { return len(ss) }
func (ss jobRows) Swap(i, j int) { ss[i], ss[j] = ss[j], ss[i] }

// in the order of:
//  1. pre-submit, category, group
//  2. post-submit, category, group
//  3. periodic, category, group
func (ss jobRows) Less(i, j int) bool {
	a, b := ss[i], ss[j]
	// pre-submit should be first
	if a.Type == prow.TypePresubmit && b.Type != prow.TypePresubmit {
		return true
	}
	// periodic should be last
	if a.Type == prow.TypePeriodic && b.Type != prow.TypePeriodic {
		return false
	}
	if a.Type == prow.TypePostsubmit && b.Type == prow.TypePeriodic {
		return true
	}
	if a.Type == prow.TypePostsubmit && b.Type == prow.TypePresubmit {
		return false
	}
	// (a.Type == TypePresubmit && b.Type == TypePresubmit) ||
	// 	(a.Type == TypePostsubmit && b.Type == TypePostsubmit) ||
	// 	(a.Type == TypePeriodic && b.Type == TypePeriodic)
	if a.Category != b.Category {
		return a.Category < b.Category
	}
	return a.Group < b.Group
}

func createJobRows(
	jobs prow.Jobs,
	all map[string]prow.Job,
	categoryToProviderToJob map[string]map[string]prow.Job,
) string {
	rows := make([]jobRow, 0, len(jobs))
	for category, providerToJob := range categoryToProviderToJob {
		row := jobRow{
			Category:               category,
			ProviderAWS:            "",
			ProviderGCP:            "",
			ProviderNotCategorized: "",
		}
		var job prow.Job
		var ok bool
		if job, ok = providerToJob[prow.ProviderAWS]; ok {
			row.Type = job.Type
			row.Group = job.Group
			row.Category = job.Category
			row.ProviderAWS = fmt.Sprintf(`<a href="%s" target="_blank">%s</a> (<a href="%s" target="_blank">status</a>)`, job.URL,
				job.ID,
				job.StatusURL,
			)
		} else {
			row.ProviderAWS = "N/A"
		}
		if job, ok = providerToJob[prow.ProviderGCP]; ok {
			row.Type = job.Type
			row.Group = job.Group
			row.Category = job.Category
			row.ProviderGCP = fmt.Sprintf(`<a href="%s" target="_blank">%s</a> (<a href="%s" target="_blank">status</a>)`, job.URL,
				job.ID,
				job.StatusURL,
			)
		}
		if job, ok = providerToJob[prow.ProviderNotCategorized]; ok {
			row.Type = job.Type
			row.Group = job.Group
			row.Category = job.Category
			row.ProviderNotCategorized = fmt.Sprintf(`<a href="%s" target="_blank">%s</a> (<a href="%s" target="_blank">status</a>)`, job.URL,
				job.ID,
				job.StatusURL,
			)
		}
		rows = append(rows, row)
	}
	tmpl := template.Must(template.New("tmplJobRows").Parse(tmplJobRows))
	buf := bytes.NewBuffer(nil)
	sort.Sort(jobRows(rows))
	if err := tmpl.Execute(buf, rows); err != nil {
		panic(err)
	}
	return html.UnescapeString(buf.String())
}

const upstreamHTMLEnd = `
</body>
</html>
`
