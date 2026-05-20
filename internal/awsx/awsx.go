package awsx

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// ListProfiles returns AWS profile names via `aws configure list-profiles`.
func ListProfiles() ([]string, error) {
	out, err := exec.Command("aws", "configure", "list-profiles").Output()
	if err != nil {
		return nil, fmt.Errorf("aws configure list-profiles: %w", err)
	}
	var profiles []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			profiles = append(profiles, line)
		}
	}
	return profiles, nil
}

// ProfileDefaultRegion reads default region from ~/.aws/config for the profile.
func ProfileDefaultRegion(profile string) string {
	out, err := exec.Command("aws", "configure", "get", "region", "--profile", profile).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// LoadConfig loads SDK config for the given profile/region.
func LoadConfig(ctx context.Context, profile, region string) (aws.Config, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithSharedConfigProfile(profile),
	}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	return config.LoadDefaultConfig(ctx, opts...)
}

type Instance struct {
	ID   string
	Name string
	AZ   string
}

// ListSSMInstances lists EC2 instances reachable via SSM (PingStatus=Online).
func ListSSMInstances(ctx context.Context, cfg aws.Config) ([]Instance, error) {
	ssmCli := ssm.NewFromConfig(cfg)
	ec2Cli := ec2.NewFromConfig(cfg)

	var ids []string
	var token *string
	for {
		out, err := ssmCli.DescribeInstanceInformation(ctx, &ssm.DescribeInstanceInformationInput{NextToken: token})
		if err != nil {
			return nil, fmt.Errorf("ssm describe-instance-information: %w", err)
		}
		for _, info := range out.InstanceInformationList {
			if info.InstanceId != nil && string(info.PingStatus) == "Online" {
				ids = append(ids, *info.InstanceId)
			}
		}
		if out.NextToken == nil {
			break
		}
		token = out.NextToken
	}

	if len(ids) == 0 {
		return nil, nil
	}

	out, err := ec2Cli.DescribeInstances(ctx, &ec2.DescribeInstancesInput{InstanceIds: ids})
	if err != nil {
		return nil, fmt.Errorf("ec2 describe-instances: %w", err)
	}

	var instances []Instance
	for _, r := range out.Reservations {
		for _, i := range r.Instances {
			ins := Instance{ID: aws.ToString(i.InstanceId)}
			if i.Placement != nil {
				ins.AZ = aws.ToString(i.Placement.AvailabilityZone)
			}
			for _, t := range i.Tags {
				if aws.ToString(t.Key) == "Name" {
					ins.Name = aws.ToString(t.Value)
				}
			}
			instances = append(instances, ins)
		}
	}
	return instances, nil
}

type DBEndpoint struct {
	ID     string // DBInstanceIdentifier or DBClusterIdentifier
	Kind   string // "instance" or "cluster"
	Engine string
	Host   string
	Port   int32
}

// ListDBEndpoints lists RDS instances + Aurora cluster (writer) endpoints.
func ListDBEndpoints(ctx context.Context, cfg aws.Config) ([]DBEndpoint, error) {
	rdsCli := rds.NewFromConfig(cfg)

	var endpoints []DBEndpoint

	{
		var marker *string
		for {
			out, err := rdsCli.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{Marker: marker})
			if err != nil {
				return nil, fmt.Errorf("rds describe-db-instances: %w", err)
			}
			for _, ins := range out.DBInstances {
				if ins.Endpoint == nil || ins.Endpoint.Address == nil {
					continue
				}
				endpoints = append(endpoints, DBEndpoint{
					ID:     aws.ToString(ins.DBInstanceIdentifier),
					Kind:   "instance",
					Engine: aws.ToString(ins.Engine),
					Host:   aws.ToString(ins.Endpoint.Address),
					Port:   aws.ToInt32(ins.Endpoint.Port),
				})
			}
			if out.Marker == nil {
				break
			}
			marker = out.Marker
		}
	}

	{
		var marker *string
		for {
			out, err := rdsCli.DescribeDBClusters(ctx, &rds.DescribeDBClustersInput{Marker: marker})
			if err != nil {
				return nil, fmt.Errorf("rds describe-db-clusters: %w", err)
			}
			for _, cl := range out.DBClusters {
				if cl.Endpoint == nil {
					continue
				}
				endpoints = append(endpoints, DBEndpoint{
					ID:     aws.ToString(cl.DBClusterIdentifier),
					Kind:   "cluster",
					Engine: aws.ToString(cl.Engine),
					Host:   aws.ToString(cl.Endpoint),
					Port:   aws.ToInt32(cl.Port),
				})
			}
			if out.Marker == nil {
				break
			}
			marker = out.Marker
		}
	}

	return endpoints, nil
}

// PortForwardCommand returns the human-readable aws CLI command for preview.
func PortForwardCommand(profile, region, instanceID, host string, remotePort, localPort int) string {
	params := fmt.Sprintf(`{"host":["%s"],"portNumber":["%d"],"localPortNumber":["%d"]}`,
		host, remotePort, localPort)
	return fmt.Sprintf(
		"aws ssm start-session --target %s --document-name AWS-StartPortForwardingSessionToRemoteHost --parameters '%s' --region %s --profile %s",
		instanceID, params, region, profile,
	)
}

// StartPortForward executes the SSM port-forwarding session, blocking until ended.
func StartPortForward(ctx context.Context, profile, region, instanceID, host string, remotePort, localPort int) error {
	params := fmt.Sprintf(`{"host":["%s"],"portNumber":["%d"],"localPortNumber":["%d"]}`,
		host, remotePort, localPort)
	cmd := exec.CommandContext(ctx,
		"aws", "ssm", "start-session",
		"--target", instanceID,
		"--document-name", "AWS-StartPortForwardingSessionToRemoteHost",
		"--parameters", params,
		"--region", region,
		"--profile", profile,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
