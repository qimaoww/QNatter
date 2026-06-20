package forward

import "testing"

func TestNftablesInitialRulesContainNatterChains(t *testing.T) {
	rules := NftablesInitialRules()

	for _, want := range []string{
		"table ip natter",
		"chain natter_dnat",
		"chain natter_snat",
		"chain natter_mark",
		"type nat hook prerouting priority -105",
		"type filter hook prerouting priority -150",
		"type nat hook postrouting priority 95",
	} {
		if !contains(rules, want) {
			t.Fatalf("initial rules missing %q:\n%s", want, rules)
		}
	}
}

func TestNftablesDNATRuleMatchesPythonFormat(t *testing.T) {
	rule := NftablesDNATRule(StartOptions{
		IP:         "198.51.100.73",
		Port:       8931,
		TargetIP:   "10.10.10.10",
		TargetPort: 51413,
	})

	want := "insert rule ip natter natter_dnat ip daddr 198.51.100.73 tcp dport 8931 dnat to 10.10.10.10:51413"
	if rule != want {
		t.Fatalf("DNAT rule = %q, want %q", rule, want)
	}
}

func TestNftablesSNATRuleUsesRouteSourceIP(t *testing.T) {
	rule := NftablesSNATRule(StartOptions{
		IP:         "198.51.100.73",
		SNATIP:     "10.10.10.1",
		Port:       8931,
		TargetIP:   "10.10.10.10",
		TargetPort: 51413,
		UDP:        true,
	})

	want := "insert rule ip natter natter_snat ip daddr 10.10.10.10 udp dport 51413 snat to 10.10.10.1"
	if rule != want {
		t.Fatalf("SNAT rule = %q, want %q", rule, want)
	}
}

func TestNftablesRouteMarkRuleMarksForwardTargetReplies(t *testing.T) {
	rule := NftablesRouteMarkRule(StartOptions{
		TargetIP:   "10.10.10.180",
		TargetPort: 25565,
	}, "0x4e34")

	want := "insert rule ip natter natter_mark ip saddr 10.10.10.180 tcp sport 25565 meta mark set 0x4e34"
	if rule != want {
		t.Fatalf("mark rule = %q, want %q", rule, want)
	}
}

func TestParseNftablesHandle(t *testing.T) {
	handle, err := ParseNftablesHandle("insert rule ip natter natter_dnat # handle 42\n")
	if err != nil {
		t.Fatalf("ParseNftablesHandle returned error: %v", err)
	}
	if handle != 42 {
		t.Fatalf("handle = %d, want 42", handle)
	}

	if _, err := ParseNftablesHandle("insert rule without handle"); err == nil {
		t.Fatal("ParseNftablesHandle accepted output without a handle")
	}
}

func contains(haystack string, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && find(haystack, needle) >= 0)
}

func find(haystack string, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
