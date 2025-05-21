package device

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// IfMap represents a mapping for a single device interface.
type IfMap struct {
	Clab     string // clab logical interface name (for output topology)
	Src      string // source/raw config interface name
	PeerNode string // peer device hostname
	PeerIf   string // peer device interface name (raw)
}

// Base provides common fields and methods for all Device types.
type Base struct {
	Kind       string
	NodeName   string
	Conf       string
	Hostname   string
	IfMaps     []IfMap
	validHosts map[string]bool
	peerMaps   map[string]map[string]string // PeerNode -> PeerIf -> Clab
}

// newBase reads the raw configuration file and returns a Base struct.
func newBase(kind, nodeName, rawPath string, validHosts map[string]bool) (*Base, error) {
	data, err := os.ReadFile(rawPath)
	if err != nil {
		return nil, fmt.Errorf("read raw-config: %w", err)
	}
	return &Base{
		Kind:       kind,
		NodeName:   nodeName,
		Conf:       string(data),
		validHosts: validHosts,
	}, nil
}

// ExtractHostname finds the hostname line from the configuration text.
func ExtractHostname(conf string) string {
	sc := bufio.NewScanner(strings.NewReader(conf))
	for sc.Scan() {
		trim := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(trim, "hostname ") {
			return strings.TrimSpace(strings.TrimPrefix(trim, "hostname"))
		}
	}
	return ""
}

// cleanConfig removes lines and blocks matching any regex in dropLines or dropBlocks.
func (b *Base) cleanConfig(dropLines, dropBlocks []*regexp.Regexp) string {
	var out []string
	skip := false
	for _, line := range splitLines(b.Conf) {
		trim := strings.TrimSpace(line)
		if skip {
			if !strings.HasPrefix(line, " ") && trim != "" {
				skip = false
			} else {
				continue
			}
		}
		if matches(trim, dropLines) {
			continue
		}
		if matches(trim, dropBlocks) {
			skip = true
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// splitLines splits a config into lines, handling CRLF/LF.
func splitLines(src string) []string {
	return regexp.MustCompile("\r?\n").Split(src, -1)
}

// findClabNameByPeer returns the clab interface name for a given peerNode and peerIf
func findClabNameByPeer(devs []Device, peerNode, peerIf string) string {
	for _, d := range devs {
		if d.GetHostname() == peerNode {
			for _, m := range d.GetIfMaps() {
				if m.Src == peerIf || m.Clab == peerIf {
					return m.Clab
				}
			}
		}
	}
	return ""
}

// rewritePhys parses the config, assigns clab names, and builds interface mapping.
// devs: all devices, for looking up peer's clab logical name
func (b *Base) rewritePhys(
	isPhys func(string) bool,
	clabIfs []string,
	peerDescRE *regexp.Regexp,
	devs []Device,
) {
	type parsedIf struct {
		ifname   string
		body     []string
		peerNode string
		peerIf   string
		phys     bool
	}
	var allIfs []parsedIf
	var globalLines []string

	sc := bufio.NewScanner(strings.NewReader(b.Conf))
	var (
		cur  parsedIf
		inIf bool
	)
	for sc.Scan() {
		line := sc.Text()
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "interface ") {
			if inIf && cur.ifname != "" {
				allIfs = append(allIfs, cur)
			}
			cur = parsedIf{ifname: strings.Fields(trim)[1]}
			inIf = true
			cur.body = []string{line}
			cur.phys = isPhys(cur.ifname)
			continue
		}
		if inIf && trim == "!" {
			cur.body = append(cur.body, line)
			allIfs = append(allIfs, cur)
			inIf = false
			cur = parsedIf{}
			continue
		}
		if inIf {
			cur.body = append(cur.body, line)
			if peerDescRE != nil && cur.peerNode == "" && cur.phys {
				if m := peerDescRE.FindStringSubmatch(line); m != nil {
					cur.peerNode = m[1]
					cur.peerIf = m[2]
				}
			}
		} else {
			globalLines = append(globalLines, line)
		}
	}
	if inIf && cur.ifname != "" {
		allIfs = append(allIfs, cur)
	}

	// Only physical interfaces with a valid peer get clab names and are mapped
	var filtered []IfMap
	clabIdx := 0
	for _, pif := range allIfs {
		if !pif.phys {
			continue
		}
		if pif.peerNode == "" || pif.peerIf == "" {
			continue
		}
		if !b.validHosts[pif.peerNode] {
			continue
		}
		if clabIdx >= len(clabIfs) {
			break
		}
		clab := clabIfs[clabIdx]
		clabIdx++
		filtered = append(filtered, IfMap{
			Clab:     clab,
			Src:      pif.ifname,
			PeerNode: pif.peerNode,
			PeerIf:   pif.peerIf,
		})
	}
	// Build peerMaps for description rewriting
	peerMaps := map[string]map[string]string{}
	for _, m := range filtered {
		if m.PeerNode != "" && m.PeerIf != "" {
			if peerMaps[m.PeerNode] == nil {
				peerMaps[m.PeerNode] = map[string]string{}
			}
			peerMaps[m.PeerNode][m.PeerIf] = m.Clab
		}
	}
	b.peerMaps = peerMaps

	// Build the output config
	clabIdx = 0
	var out []string
	out = append(out, globalLines...)
	for _, pif := range allIfs {
		if pif.phys {
			if pif.peerNode == "" || pif.peerIf == "" || !b.validHosts[pif.peerNode] || clabIdx >= len(clabIfs) {
				continue
			}
			clab := clabIfs[clabIdx]
			clabIdx++
			for i, orig := range pif.body {
				m := peerDescRE.FindStringSubmatch(strings.TrimSpace(orig))
				if m != nil {
					peerNode, peerIf := m[1], m[2]
					peerClab := findClabNameByPeer(devs, peerNode, peerIf)
					if peerClab != "" {
						indent := orig[:len(orig)-len(strings.TrimLeft(orig, " "))]
						orig = indent + "description " + peerNode + ":" + peerClab
					}
				}
				if i == 0 {
					out = append(out, "interface "+clab)
				} else if i == len(pif.body)-1 {
					out = append(out, "!")
				} else {
					out = append(out, orig)
				}
			}
		} else {
			for _, l := range pif.body {
				out = append(out, l)
			}
		}
	}
	b.IfMaps = filtered
	b.Conf = strings.Join(out, "\n")
}

// dump is a debug string for printing device state.
func (b *Base) dump(dev Device) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "NodeName=%s Kind=%s Hostname=%s\n", dev.GetNodeName(), dev.GetKind(), dev.GetHostname())
	for _, m := range dev.GetIfMaps() {
		fmt.Fprintf(&sb, "  %-22s ← %-22s  peer=%s:%s\n", m.Clab, m.Src, m.PeerNode, m.PeerIf)
	}
	return sb.String()
}

// matches returns true if s matches any regex in list.
func matches(s string, list []*regexp.Regexp) bool {
	for _, re := range list {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}
