package util

import (
	"fmt"
	"strings"

	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

const httpHeaderBoundary = ": "

// NewHTTPHeaderAPIOptions returns a slice of middleware options that adds the
// specified HTTP headers to an API request.
// Each header should be of the format `Header-Key: Header-Value`, in the same manner
// as headers are passed with `curl`-s `-H` flag.
func NewHTTPHeaderAPIOptions(headers []string) ([]func(*middleware.Stack) error, error) {
	var opts []func(*middleware.Stack) error
	for _, header := range headers {
		boundary := strings.Index(header, httpHeaderBoundary)
		if boundary == -1 {
			return nil, fmt.Errorf("malformed HTTP header: '%s'", header)
		}
		key := header[:boundary]
		val := header[boundary+len(httpHeaderBoundary):]
		opts = append(opts, smithyhttp.AddHeaderValue(key, val))
	}
	return opts, nil
}
