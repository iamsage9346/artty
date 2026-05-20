package main

import (
	"flag"
	"fmt"
	"os"
)

const usage = `usage: artty [-H|--history]

Interactively select an AWS profile, bastion EC2 (SSM), and RDS endpoint,
then open a local port-forwarding tunnel via AWS Systems Manager.

Flags:
  -H, --history    Show the AWS CLI command as you make selections
`

func main() {
	var showCmd bool
	flag.BoolVar(&showCmd, "H", false, "Show AWS commands as you make selections")
	flag.BoolVar(&showCmd, "history", false, "Same as -H")
	flag.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	flag.Parse()

	if len(flag.Args()) > 0 {
		fmt.Fprintf(os.Stderr, "unknown argument: %s\n\n%s", flag.Args()[0], usage)
		os.Exit(2)
	}

	if err := Connect(showCmd); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
