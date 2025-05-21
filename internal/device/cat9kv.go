package device

import (
	"regexp"
	"strings"
)

// clabIfsCat9kv defines the logical interface names for cisco_cat9kv
var clabIfsCat9kv = []string{
	"GigabitEthernet1/0/1", "GigabitEthernet1/0/2",
	"GigabitEthernet1/0/3", "GigabitEthernet1/0/4",
	"GigabitEthernet1/0/5", "GigabitEthernet1/0/6",
	"GigabitEthernet1/0/7", "GigabitEthernet1/0/8",
}

// peerDescRECat9kv parses lines like 'description <host>:<ifname>'
var peerDescRECat9kv = regexp.MustCompile(`^\s*description\s+(\S+):(\S+)\s*$`)

// isPhysCat9kv returns true if the interface name indicates a data port (not management)
func isPhysCat9kv(ifName string) bool {
	if len(ifName) == 0 {
		return false
	}
	if strings.HasPrefix(ifName, "GigabitEthernet0/0") {
		return false
	}
	return regexp.MustCompile(`^(GigabitEthernet|TenGigabitEthernet|TwentyFiveGigE|HundredGigE)`).MatchString(ifName)
}

// Cat9kv is a Device implementation for Cisco Cat9Kv
type Cat9kv struct{ *Base }

// NewCat9kv creates and configures a new Cat9kv device
func NewCat9kv(node, rawPath string, validHosts map[string]bool) (Device, error) {
	base, err := newBase("cisco_cat9kv", node, rawPath, validHosts)
	if err != nil {
		return nil, err
	}
	c := &Cat9kv{Base: base}
	c.Hostname = ExtractHostname(c.Conf)
	return c, nil
}

func (c *Cat9kv) GetNodeName() string { return c.NodeName }
func (c *Cat9kv) GetKind() string     { return c.Kind }
func (c *Cat9kv) GetHostname() string { return c.Hostname }
func (c *Cat9kv) GetIfMaps() []IfMap  { return c.IfMaps }
func (c *Cat9kv) CleanConfig() string {
	c.Conf = c.cleanConfig(reDropLineCat9kv, reDropBlockCat9kv)
	return c.Conf
}
func (c *Cat9kv) Dump() string { return c.dump(c) }
func (c *Cat9kv) RewritePhys(devs []Device) {
	c.rewritePhys(isPhysCat9kv, clabIfsCat9kv, peerDescRECat9kv, devs)
}

// reDropLineCat9kv and reDropBlockCat9kv are patterns for config cleanup.
var (
	reDropLineCat9kv = []*regexp.Regexp{
		regexp.MustCompile(`^aaa`),
		regexp.MustCompile(`^boot system`),
		regexp.MustCompile(`^switch \d+ provision`),
		regexp.MustCompile(`^license`),
		regexp.MustCompile(`^enable`),
		regexp.MustCompile(`^username admin`),
		regexp.MustCompile(`^ip ssh`),
		regexp.MustCompile(`^end`),
	}
	reDropBlockCat9kv = []*regexp.Regexp{
		regexp.MustCompile(`^interface +GigabitEthernet0/0\b`),
		regexp.MustCompile(`^stackwise-virtual`),
		regexp.MustCompile(`^crypto`),
		regexp.MustCompile(`^line `),
		regexp.MustCompile(`^event manager`),
	}
)
