package s3

import (
	"bytes"
	"fmt"
	"text/template"
)

func createAccessLogPolicy(accountID, bucket string) string {
	d := accessLog{
		AccountID: accountID,
		Resource:  fmt.Sprintf("arn:aws:s3:::%s/*", bucket),
	}
	tpl := template.Must(template.New("accessLogPolicyTempl").Parse(accessLogPolicyTempl))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, d); err != nil {
		panic(err)
	}
	return buf.String()
}

type accessLog struct {
	AccountID string
	Resource  string
}

// Principal element specifies the user, account, service, or other entity that is allowed or denied access to a resource
// https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-access-logs.html
// https://docs.aws.amazon.com/AmazonS3/latest/dev/s3-bucket-user-policy-specifying-principal-intro.html
//
// "Principal": {"AWS": ["{{.AccountID}}"]},
// or
// "Principal": {"AWS": ["arn:aws:iam::{{.PrincipalELBAccountID}}:root"]},
// or
// "Principal": {"AWS": "*"},
//
const accessLogPolicyTempl = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["s3:PutObject"],
      "Principal": {"AWS": ["{{.AccountID}}"]},
			"Resource": "{{.Resource}}"
	  }
  ]
}
`

// https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-access-logs.html
// https://docs.aws.amazon.com/AmazonS3/latest/dev/s3-bucket-user-policy-specifying-principal-intro.html
var regionToPrincipal = map[string]string{
	"us-east-1":      "127311923021",
	"us-east-2":      "033677994240",
	"us-west-1":      "027434742980",
	"us-west-2":      "797873946194",
	"ca-central-1":   "985666609251",
	"eu-central-1":   "054676820928",
	"eu-west-1":      "156460612806",
	"eu-west-2":      "652711504416",
	"eu-west-3":      "009996457667",
	"ap-northeast-1": "582318560864",
	"ap-northeast-2": "600734575887",
	"ap-northeast-3": "383597477331",
	"ap-southeast-1": "114774131450",
	"ap-southeast-2": "783225319266",
	"ap-south-1":     "718504428378",
	"sa-east-1":      "507241528517",
	"us-gov-west-1":  "048591011584",
	"cn-north-1":     "638102146993",
	"cn-northwest-1": "037604701340",
}
