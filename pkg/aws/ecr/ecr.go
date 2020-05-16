// Package ecr implements ECR utilities.
package ecr

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

// Check checks if the specified repository exists, and returns the repository URI + ":" + image tag.
func Check(lg *zap.Logger, svc ecriface.ECRAPI, repoAccountID string, repoName string, imgTag string) (img string, err error) {
	lg.Info("describing ECR repositories")
	repoOut, err := svc.DescribeRepositories(&ecr.DescribeRepositoriesInput{
		RegistryId:      aws.String(repoAccountID),
		RepositoryNames: aws.StringSlice([]string{repoName}),
	})
	if err != nil {
		return "", err
	}
	if len(repoOut.Repositories) != 1 {
		return "", fmt.Errorf("%q expected 1 ECR repository, got %d", repoName, len(repoOut.Repositories))
	}
	repo := repoOut.Repositories[0]
	arn := aws.StringValue(repo.RepositoryArn)
	name := aws.StringValue(repo.RepositoryName)
	uri := aws.StringValue(repo.RepositoryUri)
	lg.Info(
		"described ECR repository",
		zap.String("arn", arn),
		zap.String("name", name),
		zap.String("uri", uri),
	)
	if name != repoName {
		return "", fmt.Errorf("unexpected ECR repository name %q", name)
	}

	lg.Info("describing image", zap.String("image-tag", imgTag))
	imgOut, err := svc.DescribeImages(&ecr.DescribeImagesInput{
		RegistryId:     aws.String(repoAccountID),
		RepositoryName: aws.String(repoName),
		ImageIds: []*ecr.ImageIdentifier{
			{
				ImageTag: aws.String(imgTag),
			},
		},
	})
	if err != nil {
		lg.Warn("failed to describe image", zap.Error(err))
		return "", err
	}
	if len(imgOut.ImageDetails) == 0 {
		return "", fmt.Errorf("image tag %q not found", imgTag)
	}
	lg.Info("described images", zap.Int("images", len(imgOut.ImageDetails)))
	for i, img := range imgOut.ImageDetails {
		lg.Info("found an image",
			zap.Int("index", i),
			zap.String("requested-tag", imgTag),
			zap.Strings("returned-tags", aws.StringValueSlice(img.ImageTags)),
			zap.String("digest", aws.StringValue(img.ImageDigest)),
			zap.String("pushed-at", humanize.Time(aws.TimeValue(img.ImagePushedAt))),
			zap.String("size", humanize.Bytes(uint64(aws.Int64Value(img.ImageSizeInBytes)))),
		)
	}
	return uri + ":" + imgTag, nil
}
