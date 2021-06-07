// Package ecr implements ECR utilities.
package ecr

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

// Repository represents an ECR image.
type Repository struct {
	// Partition is used for deciding between "amazonaws.com" and "amazonaws.com.cn".
	Partition string `json:"partition"`
	// AccountID is the account ID for tester ECR image.
	// e.g. "my-app" for "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/my-app"
	AccountID string `json:"account_id"`
	// Region is the ECR repository region to pull from.
	Region string `json:"region"`
	// Name is the repositoryName for tester ECR image.
	// e.g. "my-app" for "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/my-app"
	Name string `json:"name"`
	// ImageTag is the image tag for tester ECR image.
	// e.g. "latest" for image URI "[ACCOUNT_ID].dkr.ecr.[REGION].amazonaws.com/my-app:latest"
	ImageTag string `json:"image_tag"`
}

func (repo *Repository) IsEmpty() bool {
	if repo == nil {
		return true
	}
	return repo.Partition == "" ||
		repo.AccountID == "" ||
		repo.Region == "" ||
		repo.Name == "" ||
		repo.ImageTag == ""
}

// Describe checks if the specified repository exists, and returns the repository URI + ":" + image tag.
// It returns "true" for "exists" if the repository exists.
// This method succeeds if and only if the ECR image exists and the caller is able to verify via "ecr:DescribeImages".
// The success of this operation does not guarantee the success of image pull.
// This will fail even when the image describe fails and the caller can still pull the images.
func (repo *Repository) Describe(lg *zap.Logger, svc ecriface.ECRAPI) (img string, exists bool, err error) {
	if repo == nil {
		return "", false, errors.New("empty field for describe ECR image")
	}
	if svc == nil {
		return "", false, errors.New("empty ECRAPI for describe ECR image")
	}
	if repo.Partition == "" ||
		repo.AccountID == "" ||
		repo.Region == "" ||
		repo.Name == "" ||
		repo.ImageTag == "" {
		return "", false, errors.New("empty field for describe ECR image")
	}

	// e.g. 602401143452.dkr.ecr.us-west-2.amazonaws.com/amazon-k8s-cni:v1.6.3
	ecrHost := "amazonaws.com"
	switch repo.Partition {
	case endpoints.AwsCnPartitionID:
		ecrHost = "amazonaws.com.cn"
	default:
	}
	img = fmt.Sprintf("%s.dkr.ecr.%s.%s/%s:%s", repo.AccountID, repo.Region, ecrHost, repo.Name, repo.ImageTag)

	lg.Info("describing an ECR repository",
		zap.String("repo-account-id", repo.AccountID),
		zap.String("repo-region", repo.Region),
		zap.String("repo-name", repo.Name),
		zap.String("image-tag", repo.ImageTag),
		zap.String("image", img),
	)
	repoOut, err := svc.DescribeRepositories(&ecr.DescribeRepositoriesInput{
		RegistryId:      aws.String(repo.AccountID),
		RepositoryNames: aws.StringSlice([]string{repo.Name}),
	})
	if err != nil {
		ev, ok := err.(awserr.Error)
		if !ok {
			return img, false, err
		}
		switch ev.Code() {
		case "RepositoryNotFoundException":
			lg.Warn("ECR repo not found", zap.String("error-code", ev.Code()), zap.Error(err))
			exists = false
		default:
		}
		return img, exists, err
	}
	if len(repoOut.Repositories) != 1 {
		return img, true, fmt.Errorf("%q expected 1 ECR repository, got %d", repo.Name, len(repoOut.Repositories))
	}
	rv := repoOut.Repositories[0]
	repoAccountID2 := aws.StringValue(rv.RegistryId)
	repoARN := aws.StringValue(rv.RepositoryArn)
	repoName2 := aws.StringValue(rv.RepositoryName)
	repoURI := aws.StringValue(rv.RepositoryUri)
	img = repoURI + ":" + repo.ImageTag
	lg.Info(
		"described an ECR repository",
		zap.String("repo-arn", repoARN),
		zap.String("repo-region", repo.Region),
		zap.String("repo-name", repoName2),
		zap.String("repo-uri", repoURI),
		zap.String("image", img),
	)
	if repoAccountID2 != repo.AccountID {
		return img, true, fmt.Errorf("unexpected ECR repository account ID %q (expected %q)", repoAccountID2, repo.AccountID)
	}
	if repoName2 != repo.Name {
		return img, true, fmt.Errorf("unexpected ECR repository name %q", repoName2)
	}
	if !strings.Contains(repoURI, repo.Region) {
		return img, true, fmt.Errorf("region %q not found in URI %q", repo.Region, repoURI)
	}

	lg.Info("describing images",
		zap.String("repo-name", repo.Name),
		zap.String("repo-uri", repoURI),
		zap.String("image-tag", repo.ImageTag),
	)
	imgOut, err := svc.DescribeImages(&ecr.DescribeImagesInput{
		RegistryId:     aws.String(repo.AccountID),
		RepositoryName: aws.String(repo.Name),
		ImageIds: []*ecr.ImageIdentifier{
			{
				ImageTag: aws.String(repo.ImageTag),
			},
		},
	})
	if err != nil {
		lg.Warn("failed to describe image", zap.Error(err))
		return img, true, err
	}
	if len(imgOut.ImageDetails) == 0 {
		return img, true, fmt.Errorf("image tag %q not found", repo.ImageTag)
	}
	lg.Info("described images",
		zap.String("repo-name", repo.Name),
		zap.String("image-tag", repo.ImageTag),
		zap.Int("images", len(imgOut.ImageDetails)),
	)
	for i, img := range imgOut.ImageDetails {
		lg.Info("found an image",
			zap.Int("index", i),
			zap.String("requested-tag", repo.ImageTag),
			zap.Strings("returned-tags", aws.StringValueSlice(img.ImageTags)),
			zap.String("digest", aws.StringValue(img.ImageDigest)),
			zap.String("pushed-at", fmt.Sprintf("%v", aws.TimeValue(img.ImagePushedAt))),
			zap.String("size", humanize.Bytes(uint64(aws.Int64Value(img.ImageSizeInBytes)))),
		)
	}
	return img, true, nil
}

// Check checks if the specified repository exists, and returns the repository URI + ":" + image tag.
// It returns "true" for "ok" if the repository exists.
func Check(
	lg *zap.Logger,
	svc ecriface.ECRAPI,
	partition string,
	repoAccountID string,
	repoRegion string,
	repoName string,
	imageTag string) (img string, ok bool, err error) {
	// e.g. 602401143452.dkr.ecr.us-west-2.amazonaws.com/amazon-k8s-cni:v1.6.3
	ecrHost := "amazonaws.com"
	switch partition {
	case endpoints.AwsCnPartitionID:
		ecrHost = "amazonaws.com.cn"
	default:
	}
	img = fmt.Sprintf("%s.dkr.ecr.%s.%s/%s:%s", repoAccountID, repoRegion, ecrHost, repoName, imageTag)

	lg.Info("describing an ECR repository",
		zap.String("repo-account-id", repoAccountID),
		zap.String("repo-region", repoRegion),
		zap.String("repo-name", repoName),
		zap.String("image-tag", imageTag),
		zap.String("image", img),
	)
	repoOut, err := svc.DescribeRepositories(&ecr.DescribeRepositoriesInput{
		RegistryId:      aws.String(repoAccountID),
		RepositoryNames: aws.StringSlice([]string{repoName}),
	})
	if err != nil {
		ev, ok := err.(awserr.Error)
		if !ok {
			return img, false, err
		}
		switch ev.Code() {
		case "RepositoryNotFoundException":
			lg.Warn("ECR repo not found", zap.String("error-code", ev.Code()), zap.Error(err))
			ok = false
		default:
		}
		return img, ok, err
	}
	if len(repoOut.Repositories) != 1 {
		return img, true, fmt.Errorf("%q expected 1 ECR repository, got %d", repoName, len(repoOut.Repositories))
	}
	repo := repoOut.Repositories[0]
	repoAccountID2 := aws.StringValue(repo.RegistryId)
	repoARN := aws.StringValue(repo.RepositoryArn)
	repoName2 := aws.StringValue(repo.RepositoryName)
	repoURI := aws.StringValue(repo.RepositoryUri)
	img = repoURI + ":" + imageTag
	lg.Info(
		"described an ECR repository",
		zap.String("repo-arn", repoARN),
		zap.String("repo-region", repoRegion),
		zap.String("repo-name", repoName2),
		zap.String("repo-uri", repoURI),
		zap.String("image", img),
	)
	if repoAccountID2 != repoAccountID {
		return img, true, fmt.Errorf("unexpected ECR repository account ID %q (expected %q)", repoAccountID2, repoAccountID)
	}
	if repoName2 != repoName {
		return img, true, fmt.Errorf("unexpected ECR repository name %q", repoName2)
	}
	if !strings.Contains(repoURI, repoRegion) {
		return img, true, fmt.Errorf("region %q not found in URI %q", repoRegion, repoURI)
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
		return img, true, err
	}
	if len(imgOut.ImageDetails) == 0 {
		return img, true, fmt.Errorf("image tag %q not found", imageTag)
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
	return img, true, nil
}

// Create creates an ECR repo if it does not exist.
// If the set policy fails, ECR repo creation is reverted (delete).
func Create(
	lg *zap.Logger,
	svc ecriface.ECRAPI,
	repoAccountID string,
	repoRegion string,
	repoName string,
	imgScanOnPush bool,
	imgTagMutability string,
	policyTxt string,
	setPolicyForce bool) (repoURI string, err error) {
	lg.Info("creating an ECR repository",
		zap.String("repo-account-id", repoAccountID),
		zap.String("repo-region", repoRegion),
		zap.String("repo-name", repoName),
		zap.Bool("image-scan-on-push", imgScanOnPush),
		zap.String("image-tag-mutability", imgTagMutability),
		zap.Bool("set-policy-force", setPolicyForce),
	)
	switch imgTagMutability {
	case ecr.ImageTagMutabilityMutable:
	case ecr.ImageTagMutabilityImmutable:
	default:
		return "", fmt.Errorf("invalid image tag mutability %q", imgTagMutability)
	}
	repoOut, err := svc.DescribeRepositories(&ecr.DescribeRepositoriesInput{
		RegistryId:      aws.String(repoAccountID),
		RepositoryNames: aws.StringSlice([]string{repoName}),
	})
	if err == nil {
		if len(repoOut.Repositories) != 1 {
			return "", fmt.Errorf("%q expected 1 ECR repository, got %d", repoName, len(repoOut.Repositories))
		}
		repo := repoOut.Repositories[0]
		repoAccountID2 := aws.StringValue(repo.RegistryId)
		repoARN := aws.StringValue(repo.RepositoryArn)
		repoName2 := aws.StringValue(repo.RepositoryName)
		repoURI = aws.StringValue(repo.RepositoryUri)
		lg.Info(
			"found an ECR repository",
			zap.String("repo-arn", repoARN),
			zap.String("repo-region", repoRegion),
			zap.String("repo-name", repoName2),
			zap.String("repo-uri", repoURI),
		)
		if repoAccountID2 != repoAccountID {
			return "", fmt.Errorf("unexpected ECR repository account ID %q (expected %q)", repoAccountID2, repoAccountID)
		}
		if repoName2 != repoName {
			return "", fmt.Errorf("unexpected ECR repository name %q", repoName2)
		}
		if !strings.Contains(repoURI, repoRegion) {
			return "", fmt.Errorf("region %q not found in URI %q", repoRegion, repoURI)
		}
		return repoURI, nil
	}

	ev, ok := err.(awserr.Error)
	if !ok {
		return "", err
	}
	if ev.Code() != "RepositoryNotFoundException" {
		return "", err
	}

	lg.Info("ECR repo not found; creating a new one", zap.String("error-code", ev.Code()), zap.Error(err))
	var createOutput *ecr.CreateRepositoryOutput
	createOutput, err = svc.CreateRepository(&ecr.CreateRepositoryInput{
		ImageScanningConfiguration: &ecr.ImageScanningConfiguration{
			ScanOnPush: aws.Bool(imgScanOnPush),
		},
		ImageTagMutability: aws.String(imgTagMutability),
		RepositoryName:     aws.String(repoName),
		Tags: []*ecr.Tag{
			{Key: aws.String("Kind"), Value: aws.String("aws-k8s-tester")},
		},
	})
	if err != nil {
		return "", err
	}
	repo := createOutput.Repository
	repoAccountID2 := aws.StringValue(repo.RegistryId)
	repoARN := aws.StringValue(repo.RepositoryArn)
	repoName2 := aws.StringValue(repo.RepositoryName)
	repoURI = aws.StringValue(repo.RepositoryUri)
	lg.Info(
		"created an ECR repository",
		zap.String("repo-arn", repoARN),
		zap.String("repo-region", repoRegion),
		zap.String("repo-name", repoName2),
		zap.String("repo-uri", repoURI),
	)
	if repoAccountID2 != repoAccountID {
		return "", fmt.Errorf("unexpected ECR repository account ID %q (expected %q)", repoAccountID2, repoAccountID)
	}
	if repoName2 != repoName {
		return "", fmt.Errorf("unexpected ECR repository name %q", repoName2)
	}
	if !strings.Contains(repoURI, repoRegion) {
		return "", fmt.Errorf("region %q not found in URI %q", repoRegion, repoURI)
	}

	if policyTxt != "" {
		if _, jerr := json.Marshal(policyTxt); jerr != nil {
			return "", fmt.Errorf("failed to marshal %v", jerr)
		}
		_, serr := svc.SetRepositoryPolicy(&ecr.SetRepositoryPolicyInput{
			RegistryId:     aws.String(repoAccountID),
			RepositoryName: aws.String(repoName),
			Force:          aws.Bool(setPolicyForce),
			PolicyText:     aws.String(policyTxt),
		})
		if serr != nil {
			lg.Warn("failed to set repository policy, reverting ECR repository creation", zap.Error(serr))
			if derr := Delete(lg, svc, repoAccountID, repoRegion, repoName, false); derr != nil {
				lg.Warn("failed to revert ECR repository creation", zap.Error(derr))
			}
			return "", fmt.Errorf("failed to set repostiory policy for %q (%v)", repoURI, serr)
		}
	}
	return repoURI, nil
}

// Delete deletes an ECR repo if it exists.
func Delete(
	lg *zap.Logger,
	svc ecriface.ECRAPI,
	repoAccountID string,
	repoRegion string,
	repoName string,
	force bool) (err error) {
	lg.Info("deleting an ECR repository",
		zap.String("repo-account-id", repoAccountID),
		zap.String("repo-region", repoRegion),
		zap.String("repo-name", repoName),
		zap.Bool("force", force),
	)
	repoOut, err := svc.DescribeRepositories(&ecr.DescribeRepositoriesInput{
		RegistryId:      aws.String(repoAccountID),
		RepositoryNames: aws.StringSlice([]string{repoName}),
	})
	if err != nil {
		ev, ok := err.(awserr.Error)
		if ok && ev.Code() == "RepositoryNotFoundException" {
			lg.Info("ECR repository already deleted; skipping",
				zap.String("repo-account-id", repoAccountID),
				zap.String("repo-region", repoRegion),
				zap.String("repo-name", repoName),
				zap.Error(err),
			)
			return nil
		}
		return err
	}

	if len(repoOut.Repositories) != 1 {
		return fmt.Errorf("%q expected 1 ECR repository, got %d", repoName, len(repoOut.Repositories))
	}
	repo := repoOut.Repositories[0]
	repoAccountID2 := aws.StringValue(repo.RegistryId)
	repoARN := aws.StringValue(repo.RepositoryArn)
	repoName2 := aws.StringValue(repo.RepositoryName)
	repoURI := aws.StringValue(repo.RepositoryUri)
	lg.Info(
		"found an ECR repository; deleting",
		zap.String("repo-arn", repoARN),
		zap.String("repo-region", repoRegion),
		zap.String("repo-name", repoName2),
		zap.String("repo-uri", repoURI),
	)
	if repoAccountID2 != repoAccountID {
		return fmt.Errorf("unexpected ECR repository account ID %q (expected %q)", repoAccountID2, repoAccountID)
	}
	if repoName2 != repoName {
		return fmt.Errorf("unexpected ECR repository name %q", repoName2)
	}
	if !strings.Contains(repoURI, repoRegion) {
		return fmt.Errorf("region %q not found in URI %q", repoRegion, repoURI)
	}

	_, err = svc.DeleteRepository(&ecr.DeleteRepositoryInput{
		RegistryId:     aws.String(repoAccountID),
		RepositoryName: aws.String(repoName),
		Force:          aws.Bool(force),
	})
	if err != nil {
		lg.Warn("failed to delete an ECR repository", zap.Error(err))
		return err
	}
	// confirm ECR deletion
	deleted := false
	retryStart := time.Now()
	for time.Since(retryStart) < 15*time.Minute {
		time.Sleep(5 * time.Second)

		_, derr := svc.DescribeRepositories(&ecr.DescribeRepositoriesInput{
			RegistryId:      aws.String(repoAccountID),
			RepositoryNames: aws.StringSlice([]string{repoName}),
		})
		if derr != nil {
			ev, ok := derr.(awserr.Error)
			if ok && ev.Code() == "RepositoryNotFoundException" {
				lg.Info("confirmed ECR repository has been deleted",
					zap.String("repo-account-id", repoAccountID),
					zap.String("repo-region", repoRegion),
					zap.String("repo-name", repoName),
					zap.Error(derr),
				)
				deleted = true
			}
			if !deleted {
				lg.Warn("failed to describe an ECR repository", zap.Error(derr))
			}
		}
		if deleted {
			break
		}
	}
	if !deleted {
		return fmt.Errorf("ECR %q has not been deleted", repoName)
	}

	lg.Info("deleted an ECR repository",
		zap.String("repo-account-id", repoAccountID),
		zap.String("repo-region", repoRegion),
		zap.String("repo-name", repoName),
		zap.String("repo-uri", repoURI),
		zap.Bool("force", force),
	)
	return nil
}

// SetPolicy updates the policy for an ECR repo.
func SetPolicy(
	lg *zap.Logger,
	svc ecriface.ECRAPI,
	repoAccountID string,
	repoRegion string,
	repoName string,
	policyTxt string,
	setPolicyForce bool) (repoURI string, err error) {
	if len(policyTxt) == 0 {
		return "", errors.New("empty policy")
	}

	lg.Info("setting policy for an ECR repository",
		zap.String("repo-account-id", repoAccountID),
		zap.String("repo-region", repoRegion),
		zap.String("repo-name", repoName),
		zap.Bool("set-policy-force", setPolicyForce),
	)
	repoOut, err := svc.DescribeRepositories(&ecr.DescribeRepositoriesInput{
		RegistryId:      aws.String(repoAccountID),
		RepositoryNames: aws.StringSlice([]string{repoName}),
	})
	if err != nil {
		ev, ok := err.(awserr.Error)
		if !ok {
			return "", err
		}
		if ev.Code() == "RepositoryNotFoundException" {
			lg.Warn("repository not found", zap.Error(err))
		}
		return "", err
	}

	if len(repoOut.Repositories) != 1 {
		return "", fmt.Errorf("%q expected 1 ECR repository, got %d", repoName, len(repoOut.Repositories))
	}
	repo := repoOut.Repositories[0]
	repoAccountID2 := aws.StringValue(repo.RegistryId)
	repoARN := aws.StringValue(repo.RepositoryArn)
	repoName2 := aws.StringValue(repo.RepositoryName)
	repoURI = aws.StringValue(repo.RepositoryUri)
	lg.Info(
		"found an ECR repository",
		zap.String("repo-arn", repoARN),
		zap.String("repo-region", repoRegion),
		zap.String("repo-name", repoName2),
		zap.String("repo-uri", repoURI),
	)
	if repoAccountID2 != repoAccountID {
		return "", fmt.Errorf("unexpected ECR repository account ID %q (expected %q)", repoAccountID2, repoAccountID)
	}
	if repoName2 != repoName {
		return "", fmt.Errorf("unexpected ECR repository name %q", repoName2)
	}
	if !strings.Contains(repoURI, repoRegion) {
		return "", fmt.Errorf("region %q not found in URI %q", repoRegion, repoURI)
	}

	if _, jerr := json.Marshal(policyTxt); jerr != nil {
		return "", fmt.Errorf("failed to marshal %v", jerr)
	}
	_, serr := svc.SetRepositoryPolicy(&ecr.SetRepositoryPolicyInput{
		RegistryId:     aws.String(repoAccountID),
		RepositoryName: aws.String(repoName),
		Force:          aws.Bool(setPolicyForce),
		PolicyText:     aws.String(policyTxt),
	})
	if serr != nil {
		lg.Warn("failed to set repository policy", zap.Error(serr))
		return "", fmt.Errorf("failed to set repostiory policy for %q (%v)", repoURI, serr)
	}

	lg.Info("set policy for an ECR repository",
		zap.String("repo-account-id", repoAccountID),
		zap.String("repo-region", repoRegion),
		zap.String("repo-name", repoName),
		zap.String("repo-uri", repoURI),
		zap.Bool("set-policy-force", setPolicyForce),
	)
	return repoURI, nil
}

// TODO: get auth token
// https://github.com/aws/amazon-ecs-agent/blob/master/agent/dockerclient/dockerauth/ecr.go
// automated docker push
