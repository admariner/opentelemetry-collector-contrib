// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eks // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/eks"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/eks/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "eks"

	// Environment variable that is set when running on Kubernetes.
	kubernetesServiceHostEnvVar = "KUBERNETES_SERVICE_HOST"

	clusterNameAwsEksTag     = "aws:eks:cluster-name"
	clusterNameEksTag        = "eks:cluster-name"
	kubernetesClusterNameTag = "kubernetes.io/cluster/"
)

type detectorUtils interface {
	getClusterName(ctx context.Context, logger *zap.Logger) string
	getClusterNameTagFromReservations([]types.Reservation) string
	getCloudAccountID(ctx context.Context, logger *zap.Logger) string
	getClusterVersion() (string, error)
}

type eksDetectorUtils struct {
	clientset *kubernetes.Clientset
}

// detector for EKS
type detector struct {
	utils  detectorUtils
	logger *zap.Logger
	err    error
	ra     metadata.ResourceAttributesConfig
	rb     *metadata.ResourceBuilder
}

var _ internal.Detector = (*detector)(nil)

var _ detectorUtils = (*eksDetectorUtils)(nil)

// NewDetector returns a resource detector that will detect AWS EKS resources.
func NewDetector(set processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)
	utils, err := newK8sDetectorUtils()

	return &detector{
		utils:  utils,
		logger: set.Logger,
		err:    err,
		ra:     cfg.ResourceAttributes,
		rb:     metadata.NewResourceBuilder(cfg.ResourceAttributes),
	}, nil
}

// Detect returns a Resource describing the Amazon EKS environment being run in.
func (d *detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	// Check if running on EKS.
	isEKS, err := isEKS(d.utils)
	if err != nil {
		d.logger.Debug("Unable to identify EKS environment", zap.Error(err))
		return pcommon.NewResource(), "", err
	}
	if !isEKS {
		return pcommon.NewResource(), "", nil
	}

	d.rb.SetCloudProvider(conventions.CloudProviderAWS.Value.AsString())
	d.rb.SetCloudPlatform(conventions.CloudPlatformAWSEKS.Value.AsString())
	if d.ra.CloudAccountID.Enabled {
		accountID := d.utils.getCloudAccountID(ctx, d.logger)
		d.rb.SetCloudAccountID(accountID)
	}

	if d.ra.K8sClusterName.Enabled {
		clusterName := d.utils.getClusterName(ctx, d.logger)
		d.rb.SetK8sClusterName(clusterName)
	}

	return d.rb.Emit(), conventions.SchemaURL, nil
}

func isEKS(utils detectorUtils) (bool, error) {
	if os.Getenv(kubernetesServiceHostEnvVar) == "" {
		return false, nil
	}

	clusterVersion, err := utils.getClusterVersion()
	if err != nil {
		return false, fmt.Errorf("isEks() error retrieving cluster version: %w", err)
	}
	if strings.Contains(clusterVersion, "-eks-") {
		return true, nil
	}
	return false, nil
}

func newK8sDetectorUtils() (*eksDetectorUtils, error) {
	// Get cluster configuration
	confs, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}

	// Create clientset using generated configuration
	clientset, err := kubernetes.NewForConfig(confs)
	if err != nil {
		return nil, errors.New("failed to create clientset for Kubernetes client")
	}

	return &eksDetectorUtils{clientset: clientset}, nil
}

func (e eksDetectorUtils) getClusterVersion() (string, error) {
	serverVersion, err := e.clientset.Discovery().ServerVersion()
	if err != nil {
		return "", fmt.Errorf("failed to retrieve server version: %w", err)
	}
	return serverVersion.GitVersion, nil
}

func (e eksDetectorUtils) getClusterName(ctx context.Context, logger *zap.Logger) string {
	defaultErrorMessage := "Unable to get EKS cluster name"
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		logger.Warn(defaultErrorMessage, zap.Error(err))
		return ""
	}

	imdsClient := imds.NewFromConfig(cfg)
	resp, err := imdsClient.GetRegion(ctx, &imds.GetRegionInput{})
	if err != nil {
		logger.Warn(defaultErrorMessage, zap.Error(err))
		return ""
	}

	cfg.Region = resp.Region
	ec2Client := ec2.NewFromConfig(cfg)

	instanceIdentityDocument, err := imdsClient.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		logger.Warn(defaultErrorMessage, zap.Error(err))
		return ""
	}

	instances, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{
			instanceIdentityDocument.InstanceID,
		},
	})
	if err != nil {
		logger.Warn(defaultErrorMessage, zap.Error(err))
		return ""
	}

	clusterName := e.getClusterNameTagFromReservations(instances.Reservations)
	if len(clusterName) == 0 {
		logger.Warn("Failed to detect EKS cluster name. No tag for cluster name found on EC2 instance")
		return ""
	}

	return clusterName
}

func (e eksDetectorUtils) getClusterNameTagFromReservations(reservations []types.Reservation) string {
	for _, reservation := range reservations {
		for _, instance := range reservation.Instances {
			for _, tag := range instance.Tags {
				if tag.Key == nil {
					continue
				}

				if *tag.Key == clusterNameAwsEksTag || *tag.Key == clusterNameEksTag {
					return *tag.Value
				} else if strings.HasPrefix(*tag.Key, kubernetesClusterNameTag) {
					return strings.TrimPrefix(*tag.Key, kubernetesClusterNameTag)
				}
			}
		}
	}

	return ""
}

func (e eksDetectorUtils) getCloudAccountID(ctx context.Context, logger *zap.Logger) string {
	defaultErrorMessage := "Unable to get EKS cluster account ID"
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		logger.Warn(defaultErrorMessage, zap.Error(err))
		return ""
	}

	imdsClient := imds.NewFromConfig(cfg)
	instanceIdentityDocument, err := imdsClient.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		logger.Warn(defaultErrorMessage, zap.Error(err))
		return ""
	}

	return instanceIdentityDocument.AccountID
}
