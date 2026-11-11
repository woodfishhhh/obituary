// Command winres generates and verifies the Windows release resources.
// Use the tc-hib/winres resource library directly for custom ICO sizes and
// verification without a second tool.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	root := flag.String("root", ".", "repository root")
	release := flag.String("version", "0.0.0-dev", "exact GoReleaser version")
	app := flag.String("app", "", "generate one application (default: all)")
	check := flag.String("check", "", "verify a GoReleaser dist directory instead of generating")
	flag.Parse()
	var err error
	if *check != "" {
		err = checkRelease(*root, *check)
	} else {
		err = generate(*root, *release, *app)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "winres:", err)
		os.Exit(1)
	}
}
