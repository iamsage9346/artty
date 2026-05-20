package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
)

const usage = `usage:
  artty connect \
    --profile default \
    --region ap-northeast-2 \
    --instance-id i-xxxxxxxx \
    --host <rds-endpoint> \
    --remote-port 5432 \
    --local-port 5433
`

func main() {
	if len(os.Args) < 2 || os.Args[1] != "connect" {
		fmt.Print(usage)
		os.Exit(1)
	}

	fs := flag.NewFlagSet("connect", flag.ExitOnError)
	profile := fs.String("profile", "default", "AWS profile")
	region := fs.String("region", "ap-northeast-2", "AWS region")
	instanceID := fs.String("instance-id", "", "SSM managed EC2 instance ID")
	host := fs.String("host", "", "Remote RDS endpoint")
	remotePort := fs.String("remote-port", "5432", "Remote DB port")
	localPort := fs.String("local-port", "5433", "Local forwarding port")
	_ = fs.Parse(os.Args[2:])

	if *instanceID == "" || *host == "" {
		fmt.Fprintln(os.Stderr, "missing required flags: --instance-id and --host")
		os.Exit(1)
	}

	params := fmt.Sprintf(
		`{"host":["%s"],"portNumber":["%s"],"localPortNumber":["%s"]}`,
		*host,
		*remotePort,
		*localPort,
	)

	cmd := exec.Command(
		"aws",
		"ssm",
		"start-session",
		"--target", *instanceID,
		"--document-name", "AWS-StartPortForwardingSessionToRemoteHost",
		"--parameters", params,
		"--region", *region,
		"--profile", *profile,
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	fmt.Printf("Opening tunnel localhost:%s -> %s:%s via %s\n",
		*localPort, *host, *remotePort, *instanceID)

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start tunnel: %v\n", err)
		os.Exit(1)
	}
}
