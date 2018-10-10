package accesslog

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

var albHeader = []string{
	"type",
	"timestamp",
	"elb",
	"client_port",
	"target_port",
	"request_processing_time",
	"target_processing_time",
	"response_processing_time",
	"elb_status_code",
	"target_status_code",
	"received_bytes",
	"sent_bytes",
	"request",
	"user_agent",
	"ssl_cipher",
	"ssl_protocol",
	"target_group_arn",
	"trace_id",
	"domain_name",
	"chosen_cert_arn",
	"matched_rule_priority",
	"request_creation_time",
	"actions_executed",
	"redirect_url",
}

// ALBLog is an ALB log entry.
// Defined in order from raw data.
// https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-access-logs.html
type ALBLog struct {
	Type       string `json:"type,omitempty"`
	Timestamp  string `json:"timestamp,omitempty"`
	ELB        string `json:"elb,omitempty"`
	ClientPort string `json:"client_port,omitempty"`
	TargetPort string `json:"target_port,omitempty"`

	// RequestProcessingTime is total time elapsed (in seconds, with millisecond precision)
	// from the time the load balancer received the request until the time it sent it to a target.
	RequestProcessingTime        string `json:"request_processing_time,omitempty"`
	RequestProcessingTimeSeconds float64

	// TargetProcessingTime is total time elapsed (in seconds, with millisecond precision)
	// from the time the load balancer sent the request to a target until the target
	// started to send the response headers.
	TargetProcessingTime        string `json:"target_processing_time,omitempty"`
	TargetProcessingTimeSeconds float64

	// ResponseProcessingTime is total time elapsed (in seconds, with millisecond precision)
	// from the time the load balancer received the response header from the target
	// until it started to send the response to the client. This includes both the queuing time
	// at the load balancer and the connection acquisition time from the load balancer to the client.
	ResponseProcessingTime        string `json:"response_processing_time,omitempty"`
	ResponseProcessingTimeSeconds float64

	ELBStatusCode       string `json:"elb_status_code,omitempty"`
	TargetStatusCode    string `json:"target_status_code,omitempty"`
	ReceivedBytes       string `json:"received_bytes,omitempty"`
	SentBytes           string `json:"sent_bytes,omitempty"`
	Request             string `json:"request,omitempty"`
	UserAgent           string `json:"user_agent,omitempty"`
	SSLCipher           string `json:"ssl_cipher,omitempty"`
	SSLProtocol         string `json:"ssl_protocol,omitempty"`
	TargetGroupARN      string `json:"target_group_arn,omitempty"`
	TraceID             string `json:"trace_id,omitempty"`
	DomainName          string `json:"domain_name,omitempty"`
	ChosenCertARN       string `json:"chosen_cert_arn,omitempty"`
	MatchedRulePriority string `json:"matched_rule_priority,omitempty"`
	RequestCreationTime string `json:"request_creation_time,omitempty"`
	ActionsExecuted     string `json:"actions_executed,omitempty"`
	RedirectURL         string `json:"redirect_url,omitempty"`
}

// ParseALB parses ALB access logs.
// https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-access-logs.html
func ParseALB(p string) (logs []ALBLog, err error) {
	f, err := os.OpenFile(p, os.O_RDONLY, 0444)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	logs = make([]ALBLog, 0)
	br := bufio.NewReader(f)
	for {
		l, err := br.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		var fields []string
		fields, err = splitALBLog(l)
		if err != nil {
			return nil, err
		}
		if len(fields) != 24 {
			return nil, fmt.Errorf("%s fields %d, expected 24", l, len(fields))
		}
		d := ALBLog{
			Type:                   fields[0],
			Timestamp:              fields[1],
			ELB:                    fields[2],
			ClientPort:             fields[3],
			TargetPort:             fields[4],
			RequestProcessingTime:  fields[5],
			TargetProcessingTime:   fields[6],
			ResponseProcessingTime: fields[7],
			ELBStatusCode:          fields[8],
			TargetStatusCode:       fields[9],
			ReceivedBytes:          fields[10],
			SentBytes:              fields[11],
			Request:                fields[12],
			UserAgent:              fields[13],
			SSLCipher:              fields[14],
			SSLProtocol:            fields[15],
			TargetGroupARN:         fields[16],
			TraceID:                fields[17],
			DomainName:             fields[18],
			ChosenCertARN:          fields[19],
			MatchedRulePriority:    fields[20],
			RequestCreationTime:    fields[21],
			ActionsExecuted:        fields[22],
			RedirectURL:            fields[23],
		}
		d.RequestProcessingTimeSeconds, _ = strconv.ParseFloat(d.RequestProcessingTime, 64)
		d.TargetProcessingTimeSeconds, _ = strconv.ParseFloat(d.TargetProcessingTime, 64)
		d.ResponseProcessingTimeSeconds, _ = strconv.ParseFloat(d.ResponseProcessingTime, 64)
		logs = append(logs, d)
	}
	return logs, nil
}

// ConvertALBToCSV converts ALB access log file to CSV.
func ConvertALBToCSV(p, output string) error {
	f, err := os.OpenFile(p, os.O_RDONLY, 0444)
	if err != nil {
		return err
	}
	defer f.Close()

	rows := make([][]string, 0)
	br := bufio.NewReader(f)
	for {
		l, err := br.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		var row []string
		row, err = splitALBLog(l)
		if err != nil {
			return err
		}
		if len(row) != 24 {
			return fmt.Errorf("%s fields %d, expected 24", l, len(row))
		}
		rows = append(rows, row)
	}
	return toCSV(albHeader, rows, output)
}

func splitALBLog(l string) (fields []string, err error) {
	rd := csv.NewReader(strings.NewReader(l))

	// in case that rows have different number of fields
	rd.FieldsPerRecord = -1
	rd.Comma = ' '
	rd.TrailingComma = true
	rd.TrimLeadingSpace = true
	rd.LazyQuotes = true

	var rows [][]string
	rows, err = rd.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, fmt.Errorf("expected one row from line, got %d", len(rows))
	}
	return rows[0], nil
}
