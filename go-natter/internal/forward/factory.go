package forward

import "fmt"

type Forwarder interface {
	Start(StartOptions) error
	Stop() error
}

type FactoryOptions struct {
	Method     string
	BindValue  string
	BindPort   int
	TargetIP   string
	TargetPort int
}

func ResolveMethod(options FactoryOptions) string {
	if options.Method != "" && options.Method != "auto" {
		return options.Method
	}

	targetIP := options.TargetIP
	if targetIP == "" {
		targetIP = "0.0.0.0"
	}
	if targetIP == "0.0.0.0" && options.TargetPort == 0 {
		if (options.BindValue == "" || options.BindValue == "0.0.0.0") && options.BindPort == 0 {
			return "test"
		}
		return "none"
	}
	return "socket"
}

func NewForwarder(method string) (Forwarder, error) {
	switch method {
	case "none":
		return None{}, nil
	case "test":
		return &TestServer{}, nil
	case "socket":
		return &SocketForwarder{}, nil
	case "nftables":
		return &NftablesForwarder{}, nil
	case "nftables-snat":
		return &NftablesForwarder{SNAT: true}, nil
	default:
		return nil, fmt.Errorf("unsupported forward method %q", method)
	}
}
