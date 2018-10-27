package eksconfig

import (
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"time"
)

// genTag generates a tag for cluster name, CloudFormation, and S3 bucket.
// Note that this would be used as S3 bucket name to upload tester logs.
func genTag() string {
	// use UTC time for everything
	now := time.Now().UTC()
	return fmt.Sprintf("awstester-eks-%d%02d%02d", now.Year(), now.Month(), now.Day())
}

func genClusterName(tag string) string {
	h, _ := os.Hostname()
	h = strings.TrimSpace(reg.ReplaceAllString(h, ""))
	if len(h) > 12 {
		h = h[:12]
	}
	name := tag
	if len(name) > 0 {
		name += "-"
	}
	return fmt.Sprintf("%s%s-%s", name, h, randString(7))
}

var reg *regexp.Regexp

func init() {
	var err error
	reg, err = regexp.Compile("[^a-zA-Z]+")
	if err != nil {
		panic(err)
	}
}

const ll = "0123456789abcdefghijklmnopqrstuvwxyz"

func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		rand.Seed(time.Now().UTC().UnixNano())
		b[i] = ll[rand.Intn(len(ll))]
	}
	return string(b)
}
