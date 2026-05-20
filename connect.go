package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/iamsage9346/artty/internal/awsx"
	"github.com/iamsage9346/artty/internal/storage"
)

var commonRegions = []string{
	"ap-northeast-2",
	"ap-northeast-1",
	"ap-southeast-1",
	"ap-southeast-2",
	"us-east-1",
	"us-east-2",
	"us-west-2",
	"eu-west-1",
	"eu-central-1",
}

// Connect runs the full interactive RDS port-forwarding flow.
func Connect(showCmd bool) error {
	ctx := context.Background()

	if last, ok := storage.Load(); ok {
		var useLast bool
		err := huh.NewConfirm().
			Title("Use last session?").
			Description(fmt.Sprintf("%s | %s | %s | %s:%d → localhost:%d",
				last.Profile, last.Region, last.InstanceID, last.RDSHost, last.RDSPort, last.LocalPort)).
			Affirmative("Yes").
			Negative("No, start fresh").
			Value(&useLast).
			Run()
		if err != nil {
			return err
		}
		if useLast {
			return runTunnel(ctx, last, showCmd)
		}
	}

	// Step 1: AWS profile
	profiles, err := awsx.ListProfiles()
	if err != nil {
		return err
	}
	if len(profiles) == 0 {
		return errors.New("no AWS profiles found. Run 'aws configure --profile <name>' first")
	}
	var profile string
	if err := huh.NewSelect[string]().
		Title("AWS Profile").
		Options(toOpts(profiles)...).
		Value(&profile).
		Run(); err != nil {
		return err
	}
	preview(showCmd, fmt.Sprintf("aws ssm start-session --profile %s", profile))

	// Step 2: Region (pre-select profile's default if any)
	defaultRegion := awsx.ProfileDefaultRegion(profile)
	regions := append([]string{}, commonRegions...)
	if defaultRegion != "" && !contains(regions, defaultRegion) {
		regions = append([]string{defaultRegion}, regions...)
	}
	region := defaultRegion
	if region == "" {
		region = regions[0]
	}
	if err := huh.NewSelect[string]().
		Title("Region").
		Options(toOpts(regions)...).
		Value(&region).
		Run(); err != nil {
		return err
	}
	preview(showCmd, fmt.Sprintf("aws ssm start-session --profile %s --region %s", profile, region))

	// Step 3: load SDK config, list SSM EC2s
	cfg, err := awsx.LoadConfig(ctx, profile, region)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}

	fmt.Println("→ Listing SSM-eligible EC2 instances...")
	instances, err := awsx.ListSSMInstances(ctx, cfg)
	if err != nil {
		return err
	}
	if len(instances) == 0 {
		return errors.New("no SSM-eligible EC2 instances found in this region (check SSM agent + IAM role)")
	}
	instOpts := make([]huh.Option[string], 0, len(instances))
	for _, ins := range instances {
		label := ins.ID
		if ins.Name != "" {
			label = ins.Name + " — " + ins.ID
		}
		if ins.AZ != "" {
			label += " (" + ins.AZ + ")"
		}
		instOpts = append(instOpts, huh.NewOption(label, ins.ID))
	}
	var instanceID string
	if err := huh.NewSelect[string]().
		Title("Bastion EC2 (SSM)").
		Options(instOpts...).
		Value(&instanceID).
		Run(); err != nil {
		return err
	}
	preview(showCmd, fmt.Sprintf(
		"aws ssm start-session --profile %s --region %s --target %s --document-name AWS-StartPortForwardingSessionToRemoteHost",
		profile, region, instanceID))

	// Step 4: list RDS endpoints
	fmt.Println("→ Listing RDS instances and Aurora clusters...")
	endpoints, err := awsx.ListDBEndpoints(ctx, cfg)
	if err != nil {
		return err
	}
	if len(endpoints) == 0 {
		return errors.New("no RDS instances or Aurora clusters found in this region")
	}
	epMap := make(map[string]awsx.DBEndpoint, len(endpoints))
	epOpts := make([]huh.Option[string], 0, len(endpoints))
	for _, e := range endpoints {
		key := e.ID + "@" + e.Host + ":" + strconv.Itoa(int(e.Port))
		label := fmt.Sprintf("%s  [%s/%s]  %s:%d", e.ID, e.Engine, e.Kind, e.Host, e.Port)
		epMap[key] = e
		epOpts = append(epOpts, huh.NewOption(label, key))
	}
	var rdsKey string
	if err := huh.NewSelect[string]().
		Title("RDS endpoint").
		Options(epOpts...).
		Value(&rdsKey).
		Run(); err != nil {
		return err
	}
	chosen := epMap[rdsKey]

	// Step 5: local port (default 5433, scan up if busy)
	defaultLocal := 5433
	for ; defaultLocal < 5500; defaultLocal++ {
		if !isPortInUse(defaultLocal) {
			break
		}
	}
	defaultLocalStr := strconv.Itoa(defaultLocal)
	var localStr string
	if err := huh.NewInput().
		Title("Local port").
		Placeholder(defaultLocalStr).
		Description("Press Enter to use " + defaultLocalStr).
		Value(&localStr).
		Run(); err != nil {
		return err
	}
	localStr = strings.TrimSpace(localStr)
	if localStr == "" {
		localStr = defaultLocalStr
	}
	localPort, err := strconv.Atoi(localStr)
	if err != nil || localPort <= 0 || localPort > 65535 {
		return fmt.Errorf("invalid local port: %s", localStr)
	}

	// Step 6: confirm
	cmd := awsx.PortForwardCommand(profile, region, instanceID, chosen.Host, int(chosen.Port), localPort)
	fmt.Println()
	fmt.Println("Command to run:")
	fmt.Println("  " + cmd)
	fmt.Println()
	var confirm bool
	if err := huh.NewConfirm().
		Title(fmt.Sprintf("Open tunnel localhost:%d → %s:%d via %s?", localPort, chosen.Host, chosen.Port, instanceID)).
		Affirmative("Yes").
		Negative("Cancel").
		Value(&confirm).
		Run(); err != nil {
		return err
	}
	if !confirm {
		fmt.Println("cancelled")
		return nil
	}

	sess := storage.LastSession{
		Profile:    profile,
		Region:     region,
		InstanceID: instanceID,
		RDSHost:    chosen.Host,
		RDSPort:    int(chosen.Port),
		LocalPort:  localPort,
	}
	if err := storage.Save(sess); err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not save session:", err)
	}
	return runTunnel(ctx, sess, showCmd)
}

func runTunnel(ctx context.Context, s storage.LastSession, showCmd bool) error {
	if showCmd {
		preview(true, awsx.PortForwardCommand(s.Profile, s.Region, s.InstanceID, s.RDSHost, s.RDSPort, s.LocalPort))
	}
	fmt.Printf("\nOpening tunnel localhost:%d → %s:%d via %s\n\n",
		s.LocalPort, s.RDSHost, s.RDSPort, s.InstanceID)
	return awsx.StartPortForward(ctx, s.Profile, s.Region, s.InstanceID, s.RDSHost, s.RDSPort, s.LocalPort)
}

func preview(show bool, cmd string) {
	if !show {
		return
	}
	fmt.Println()
	fmt.Println("  $ " + cmd)
	fmt.Println()
}

func toOpts(values []string) []huh.Option[string] {
	opts := make([]huh.Option[string], len(values))
	for i, v := range values {
		opts[i] = huh.NewOption(v, v)
	}
	return opts
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func isPortInUse(port int) bool {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return true
	}
	_ = ln.Close()
	return false
}
