package config

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
)

type STUNServer struct {
	Host string
	Port int
}

type Config struct {
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
		KeepAliveInterval: 15,
		BindValue:         "0.0.0.0",
		TargetIP:          "0.0.0.0",
	}
	var stunValues stringList

	fs := flag.NewFlagSet("natter", flag.ContinueOnError)
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
	}

	return cfg, nil
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

	host := value
	port := 3478
	if idx := strings.LastIndex(value, ":"); idx >= 0 {
		host = value[:idx]
		parsed, err := strconv.Atoi(value[idx+1:])
		if err != nil || parsed < 1 || parsed > 65535 {
			return STUNServer{}, fmt.Errorf("invalid STUN server port %q", value)
		}
		port = parsed
	}
	if host == "" {
		return STUNServer{}, fmt.Errorf("invalid STUN server host %q", value)
	}

	return STUNServer{Host: host, Port: port}, nil
}
