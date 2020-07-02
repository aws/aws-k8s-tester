// Package ecr implements ECR utilities.
package ecr

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

// Check checks if the specified repository exists, and returns the repository URI + ":" + image tag.
func Check(lg *zap.Logger, svc ecriface.ECRAPI, repoAccountID string, repoRegion string, repoName string, imageTag string) (img string, err error) {
	lg.Info("describing ECR repositories",
		zap.String("repo-account-id", repoAccountID),
		zap.String("repo-region", repoRegion),
		zap.String("repo-name", repoName),
		zap.String("image-tag", imageTag),
	)
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
	repoARN := aws.StringValue(repo.RepositoryArn)
	repoName2 := aws.StringValue(repo.RepositoryName)
	repoURI := aws.StringValue(repo.RepositoryUri)
	img = repoURI + ":" + imageTag
	lg.Info(
		"described ECR repository",
		zap.String("repo-arn", repoARN),
		zap.String("repo-region", repoRegion),
		zap.String("repo-name", repoName2),
		zap.String("repo-uri", repoURI),
		zap.String("img", img),
	)

	if repoName2 != repoName {
		return "", fmt.Errorf("unexpected ECR repository name %q", repoName2)
	}
	if !strings.Contains(repoURI, repoRegion) {
		return "", fmt.Errorf("region %q not found in URI %q", repoRegion, repoURI)
	}

	lg.Info("describing images",
		zap.String("repo-name", repoName),
		zap.String("repo-uri", repoURI),
		zap.String("image-tag", imageTag),
	)
	imgOut, err := svc.DescribeImages(&ecr.DescribeImagesInput{
		RegistryId:     aws.String(repoAccountID),
		RepositoryName: aws.String(repoName),
		ImageIds: []*ecr.ImageIdentifier{
			{
				ImageTag: aws.String(imageTag),
			},
		},
	})
	if err != nil {
		lg.Warn("failed to describe image", zap.Error(err))
		return "", err
	}
	if len(imgOut.ImageDetails) == 0 {
		return "", fmt.Errorf("image tag %q not found", imageTag)
	}
	lg.Info("described images",
		zap.String("repo-name", repoName),
		zap.String("image-tag", imageTag),
		zap.Int("images", len(imgOut.ImageDetails)),
	)
	for i, img := range imgOut.ImageDetails {
		lg.Info("found an image",
			zap.Int("index", i),
			zap.String("requested-tag", imageTag),
			zap.Strings("returned-tags", aws.StringValueSlice(img.ImageTags)),
			zap.String("digest", aws.StringValue(img.ImageDigest)),
			zap.String("pushed-at", fmt.Sprintf("%v", aws.TimeValue(img.ImagePushedAt))),
			zap.String("size", humanize.Bytes(uint64(aws.Int64Value(img.ImageSizeInBytes)))),
		)
	}

	return img, nil
}
