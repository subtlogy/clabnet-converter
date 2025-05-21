package device

import (
	"fmt"
	"regexp"
)

// clabIfsN9kv defines the logical interface names for cisco_n9kv
var clabIfsN9kv = func() []string {
	l := make([]string, 64)
	for i := range l {
		l[i] = fmt.Sprintf("Ethernet1/%d", i+1)
	}
	return l
}()

// peerDescREN9kv parses lines like 'description <host>:<ifname>'
var peerDescREN9kv = regexp.MustCompile(`^\s*description\s+(\S+):(\S+)\s*$`)

// isPhysN9kv returns true if the interface name is a data port (Ethernet)
func isPhysN9kv(ifName string) bool {
	return regexp.MustCompile(`^Ethernet`).MatchString(ifName)
}

// N9kv is a Device implementation for Cisco N9Kv
type N9kv struct{ *Base }

// NewN9kv creates and configures a new N9kv device
func NewN9kv(node, rawPath string, validHosts map[string]bool) (Device, error) {
	base, err := newBase("cisco_n9kv", node, rawPath, validHosts)
	if err != nil {
		return nil, err
	}
	n := &N9kv{Base: base}
	n.Hostname = ExtractHostname(n.Conf)
	return n, nil
}

func (n *N9kv) GetNodeName() string { return n.NodeName }
func (n *N9kv) GetKind() string     { return n.Kind }
func (n *N9kv) GetHostname() string { return n.Hostname }
func (n *N9kv) GetIfMaps() []IfMap  { return n.IfMaps }
func (n *N9kv) CleanConfig() string {
	n.Conf = n.cleanConfig(reDropLineN9kv, reDropBlockN9kv)
	return n.Conf
}
func (n *N9kv) Dump() string { return n.dump(n) }
func (n *N9kv) RewritePhys(devs []Device) {
	n.rewritePhys(isPhysN9kv, clabIfsN9kv, peerDescREN9kv, devs)
}

// reDropLineN9kv and reDropBlockN9kv are patterns for config cleanup.
var (
	reDropLineN9kv = []*regexp.Regexp{
		regexp.MustCompile(`^feature qos`),
		regexp.MustCompile(`^username admin`),
		regexp.MustCompile(`^end`),
	}
	reDropBlockN9kv = []*regexp.Regexp{
		regexp.MustCompile(`^vdc`),
		regexp.MustCompile(`^aaa`),
		regexp.MustCompile(`^interface port-channel\d+\.\d`),
		regexp.MustCompile(`^interface mgmt0`),
	}
)
