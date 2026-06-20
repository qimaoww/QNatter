package config

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"qnatter-openwrt/go-qnatter/internal/endpoint"
)

type STUNServer struct {
	Host string
	Port int
}

type Config struct {
	InstanceID        string
	RouteMark         string
	RouteTable        string
	RoutePriority     string
	UDP               bool
	Verbose           bool
	ExitWhenChanged   bool
	UPnP              bool
	KeepAliveInterval int
	STUNServers       []STUNServer
	KeepAliveServer   string
	BindValue         string
	BindPort          int
	ForwardMethod     string
	TargetIP          string
	TargetPort        int
	RetryTarget       bool
	NotifyPath        string
	Check             bool
}

type stringList []string

func (s *stringList) String() string {
	return strings.Join(*s, ",")
}

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func ParseArgs(args []string) (Config, error) {
	cfg := Config{
		InstanceID:        os.Getenv("QNATTER_INSTANCE"),
		RouteMark:         os.Getenv("QNATTER_ROUTE_MARK"),
		RouteTable:        os.Getenv("QNATTER_ROUTE_TABLE"),
		RoutePriority:     os.Getenv("QNATTER_ROUTE_PRIORITY"),
		KeepAliveInterval: 15,
		BindValue:         "0.0.0.0",
		TargetIP:          "0.0.0.0",
	}
	var stunValues stringList

	fs := flag.NewFlagSet("qnatter", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&cfg.Verbose, "v", false, "")
	fs.BoolVar(&cfg.ExitWhenChanged, "q", false, "")
	fs.BoolVar(&cfg.UDP, "u", false, "")
	fs.BoolVar(&cfg.UPnP, "U", false, "")
	fs.BoolVar(&cfg.Check, "check", false, "")
	fs.IntVar(&cfg.KeepAliveInterval, "k", cfg.KeepAliveInterval, "")
	fs.Var(&stunValues, "s", "")
	fs.StringVar(&cfg.KeepAliveServer, "h", "", "")
	fs.StringVar(&cfg.NotifyPath, "e", "", "")
	fs.StringVar(&cfg.BindValue, "i", cfg.BindValue, "")
	fs.IntVar(&cfg.BindPort, "b", 0, "")
	fs.StringVar(&cfg.ForwardMethod, "m", "", "")
	fs.StringVar(&cfg.TargetIP, "t", cfg.TargetIP, "")
	fs.IntVar(&cfg.TargetPort, "p", 0, "")
	fs.BoolVar(&cfg.RetryTarget, "r", false, "")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if err := validateConfig(&cfg); err != nil {
		return Config{}, err
	}

	if len(stunValues) == 0 {
		stunValues = defaultSTUNServers(cfg.UDP)
	}

	servers := make([]STUNServer, 0, len(stunValues))
	for _, item := range stunValues {
		server, err := parseSTUNServer(item)
		if err != nil {
			return Config{}, err
		}
		servers = append(servers, server)
	}
	cfg.STUNServers = servers

	if cfg.KeepAliveServer == "" {
		if cfg.UDP {
			cfg.KeepAliveServer = "119.29.29.29"
		} else {
			cfg.KeepAliveServer = "www.baidu.com"
		}
	} else if _, _, err := parseHostPortDefault(cfg.KeepAliveServer, 0); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func validateConfig(cfg *Config) error {
	if err := validateRoutePolicy(cfg); err != nil {
		return err
	}
	if cfg.KeepAliveInterval <= 0 {
		return fmt.Errorf("not a positive integer: %d", cfg.KeepAliveInterval)
	}
	if !validPort(cfg.BindPort) {
		return fmt.Errorf("invalid port number: %d", cfg.BindPort)
	}
	if !validPort(cfg.TargetPort) {
		return fmt.Errorf("invalid port number: %d", cfg.TargetPort)
	}
	if cfg.NotifyPath != "" {
		if info, err := os.Stat(cfg.NotifyPath); err != nil || info.IsDir() {
			return fmt.Errorf("file not found: %s", cfg.NotifyPath)
		}
	}
	if normalized, ok := normalizeIPv4(cfg.BindValue); ok {
		cfg.BindValue = normalized
	} else if looksLikeIPv4Literal(cfg.BindValue) {
		return fmt.Errorf("invalid IP address: %s", cfg.BindValue)
	}
	normalizedTarget, ok := normalizeIPv4(cfg.TargetIP)
	if !ok {
		return fmt.Errorf("invalid IP address: %s", cfg.TargetIP)
	}
	cfg.TargetIP = normalizedTarget
	return nil
}

func validateRoutePolicy(cfg *Config) error {
	if cfg.RouteMark == "" && cfg.RouteTable == "" && cfg.RoutePriority == "" {
		return nil
	}
	if cfg.RouteMark == "" || cfg.RouteTable == "" || cfg.RoutePriority == "" {
		return fmt.Errorf("incomplete route policy")
	}
	if _, err := strconv.ParseUint(cfg.RouteMark, 0, 32); err != nil {
		return fmt.Errorf("invalid route mark: %s", cfg.RouteMark)
	}
	if _, err := strconv.ParseUint(cfg.RouteTable, 10, 32); err != nil {
		return fmt.Errorf("invalid route table: %s", cfg.RouteTable)
	}
	if _, err := strconv.ParseUint(cfg.RoutePriority, 10, 32); err != nil {
		return fmt.Errorf("invalid route priority: %s", cfg.RoutePriority)
	}
	return nil
}

func validPort(port int) bool {
	return port >= 0 && port <= 65535
}

func looksLikeIPv4Literal(value string) bool {
	if value == "" {
		return false
	}
	hasDot := false
	for _, char := range value {
		if char == '.' {
			hasDot = true
			continue
		}
		if char < '0' || char > '9' {
			return false
		}
	}
	return hasDot
}

func defaultSTUNServers(udp bool) []string {
	base := []string{
		"fwa.lifesizecloud.com",
		"global.turn.twilio.com",
		"turn.cloudflare.com",
		"stun.nextcloud.com",
		"stun.freeswitch.org",
		"stun.voip.blackberry.com",
		"stun.sipnet.com",
		"stun.radiojar.com",
		"stun.sonetel.com",
		"stun.telnyx.com",
	}
	if udp {
		return append([]string{
			"stun.miwifi.com",
			"stun.chat.bilibili.com",
			"stun.hitv.com",
			"stun.cdnbye.com",
			"stun.douyucdn.cn:18000",
		}, base...)
	}
	return append(base, "turn.cloud-rtc.com:80")
}

func parseSTUNServer(value string) (STUNServer, error) {
	if value == "" {
		return STUNServer{}, fmt.Errorf("empty STUN server")
	}
	value = trimSTUNScheme(value)

	host, port, err := endpoint.SplitHostPortDefault(value, 3478)
	if err != nil {
		return STUNServer{}, fmt.Errorf("invalid STUN server %q", value)
	}

	return STUNServer{Host: host, Port: port}, nil
}

func trimSTUNScheme(value string) string {
	for _, scheme := range []string{"tcp://", "udp://"} {
		if strings.HasPrefix(value, scheme) {
			return strings.TrimPrefix(value, scheme)
		}
	}
	return value
}

func parseHostPortDefault(value string, defaultPort int) (string, int, error) {
	return endpoint.SplitHostPortDefault(value, defaultPort)
}

func normalizeIPv4(value string) (string, bool) {
	parts := strings.Split(value, ".")
	if len(parts) < 1 || len(parts) > 4 {
		return "", false
	}
	var nums [4]uint64
	for i, part := range parts {
		if part == "" {
			return "", false
		}
		n, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return "", false
		}
		nums[i] = n
	}
	var ip uint32
	switch len(parts) {
	case 1:
		ip = uint32(nums[0])
	case 2:
		if nums[0] > 255 || nums[1] > 0xffffff {
			return "", false
		}
		ip = uint32(nums[0]<<24 | nums[1])
	case 3:
		if nums[0] > 255 || nums[1] > 255 || nums[2] > 0xffff {
			return "", false
		}
		ip = uint32(nums[0]<<24 | nums[1]<<16 | nums[2])
	case 4:
		for _, n := range nums {
			if n > 255 {
				return "", false
			}
		}
		ip = uint32(nums[0]<<24 | nums[1]<<16 | nums[2]<<8 | nums[3])
	}
	raw := make(net.IP, 4)
	binary.BigEndian.PutUint32(raw, ip)
	return raw.String(), true
}
