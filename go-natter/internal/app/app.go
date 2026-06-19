package app

import (
	"fmt"
	"io"
	"strings"

	"natter-openwrt/go-natter/internal/config"
)

const Version = "2.2.1-go"

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	for _, arg := range args {
		if arg == "--version" || arg == "-V" {
			fmt.Fprintf(stdout, "Natter Go %s\n", Version)
			return 0
		}
	}

	cfg, err := config.ParseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "natter: %v\n", err)
		return 2
	}

	if cfg.Check {
		fmt.Fprintln(stdout, "check: ok")
		fmt.Fprintf(stdout, "bind=%s:%d\n", cfg.BindValue, cfg.BindPort)
		fmt.Fprintf(stdout, "stun=%s\n", formatSTUNServers(cfg.STUNServers))
		fmt.Fprintf(stdout, "method=%s\n", cfg.ForwardMethod)
		return 0
	}

	fmt.Fprintln(stderr, "natter: Go runtime is not enabled for production mapping yet")
	return 2
}

func formatSTUNServers(servers []config.STUNServer) string {
	items := make([]string, 0, len(servers))
	for _, server := range servers {
		items = append(items, fmt.Sprintf("%s:%d", server.Host, server.Port))
	}
	return strings.Join(items, ",")
}
