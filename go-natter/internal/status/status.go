package status

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
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

func RuntimeSlug(name string) string {
	if name == "" {
		name = "default"
	}
	var out strings.Builder
	for _, char := range name {
		if char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '_' {
			out.WriteRune(char)
		} else if char < 128 {
			out.WriteString(fmt.Sprintf("_x%02x", char))
		} else {
			out.WriteByte('_')
		}
	}
	if out.Len() == 0 {
		return "default"
	}
	return out.String()
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

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(payload); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(path, buf.Bytes(), 0o644)
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	removeTmp := true
	defer func() {
		if removeTmp {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	removeTmp = false
	return nil
}
