package status

import (
	"encoding/json"
	"net/netip"
	"os"
	"path/filepath"
	"time"
)

type Mapping struct {
	Instance string
	Protocol string
	Inner    netip.AddrPort
	Outer    netip.AddrPort
	Message  string
	Now      func() string
}

type mappingJSON struct {
	Instance  string `json:"instance"`
	Protocol  string `json:"protocol"`
	InnerIP   string `json:"inner_ip"`
	InnerPort uint16 `json:"inner_port"`
	OuterIP   string `json:"outer_ip"`
	OuterPort uint16 `json:"outer_port"`
	UpdatedAt string `json:"updated_at"`
	Message   string `json:"message"`
}

func WriteMapping(path string, mapping Mapping) error {
	if mapping.Instance == "" {
		mapping.Instance = "default"
	}
	if mapping.Message == "" {
		mapping.Message = "mapped"
	}

	now := mapping.Now
	if now == nil {
		now = func() string {
			return time.Now().Format("2006-01-02 15:04:05")
		}
	}

	payload := mappingJSON{
		Instance:  mapping.Instance,
		Protocol:  mapping.Protocol,
		InnerIP:   mapping.Inner.Addr().String(),
		InnerPort: mapping.Inner.Port(),
		OuterIP:   mapping.Outer.Addr().String(),
		OuterPort: mapping.Outer.Port(),
		UpdatedAt: now(),
		Message:   mapping.Message,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}
