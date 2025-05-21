package main

import (
	"flag"
	"fmt"
	"gopkg.in/yaml.v3"
	"my-lab-proj/internal/device"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// rawNode represents a node as described in the input YAML file.
type rawNode struct {
	Kind      string `yaml:"kind"`
	Image     string `yaml:"image"`
	RawConfig string `yaml:"raw_config"`
}

// labFile represents the parsed lab.yml structure.
type labFile struct {
	Name     string `yaml:"name"`
	Topology struct {
		Nodes map[string]rawNode `yaml:"nodes"`
	} `yaml:"topology"`
}

// topoLink represents a connection between two endpoints for the output topology.
type topoLink struct {
	Endpoints []string `yaml:"endpoints"`
}

// deviceFactory maps kind names to constructor functions.
var deviceFactory = map[string]func(string, string, map[string]bool) (device.Device, error){
	"cisco_cat9kv": device.NewCat9kv,
	"cisco_n9kv":   device.NewN9kv,
}

func main() {
	// Flag for topology output filename
	var topoOut string
	flag.StringVar(&topoOut, "t", "topology.yml", "Output topology YAML file (default: topology.yml)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: labbuild [-t output_topology.yml] <lab.yml>")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}
	labPath := flag.Arg(0)

	lab, projDir := readLab(labPath)

	// Gather valid hostnames (for peer validation)
	validHosts := extractValidHostnames(lab, projDir)

	cfgDir := filepath.Join(projDir, "new_configs")
	must(os.MkdirAll(cfgDir, 0o755))

	// Build device objects for all nodes.
	devs := buildDevices(lab, projDir, cfgDir, validHosts)

	// Validate and resolve peers.
	device.ResolvePeers(devs)

	// Output the topology YAML file.
	writeTopology(filepath.Join(projDir, topoOut), lab, devs)

	// Debug print (optional)
	debugDump(devs)
}

// readLab parses the lab YAML file and returns its struct and base directory.
func readLab(path string) (lab labFile, dir string) {
	data, err := os.ReadFile(path)
	must(err)
	must(yaml.Unmarshal(data, &lab))
	return lab, filepath.Dir(path)
}

// extractValidHostnames extracts hostnames for all nodes (for peer validation).
func extractValidHostnames(lab labFile, projDir string) map[string]bool {
	hosts := make(map[string]bool)
	for _, n := range lab.Topology.Nodes {
		raw, _ := os.ReadFile(filepath.Join(projDir, n.RawConfig))
		if h := device.ExtractHostname(string(raw)); h != "" {
			hosts[h] = true
		}
	}
	return hosts
}

// buildDevices creates and configures all device objects.
func buildDevices(lab labFile, projDir, cfgDir string, valid map[string]bool) []device.Device {
	devs := make([]device.Device, 0, len(lab.Topology.Nodes))
	devMap := map[string]device.Device{}
	for name, n := range lab.Topology.Nodes {
		factory, ok := deviceFactory[n.Kind]
		if !ok {
			panic(fmt.Sprintf("Unsupported kind: %s", n.Kind))
		}
		dev, err := factory(name, filepath.Join(projDir, n.RawConfig), valid)
		must(err)
		devs = append(devs, dev)
		devMap[name] = dev
	}
	for _, dev := range devs {
		dev.(interface {
			RewritePhys([]device.Device)
		}).RewritePhys(devs)
		saveConfig(dev, cfgDir)
	}
	return devs
}

// saveConfig writes a device's configuration to a file.
func saveConfig(d device.Device, dir string) {
	out := filepath.Join(dir, d.GetNodeName()+".cfg")
	cfg := d.CleanConfig()
	if !strings.HasSuffix(cfg, "\n") {
		cfg += "\n"
	}
	must(os.WriteFile(out, []byte(cfg), 0o644))
	fmt.Printf("[+] wrote %s\n", out)
}

// writeTopology outputs the final topology YAML (for clab).
func writeTopology(outPath string, lab labFile, devs []device.Device) {
	f, err := os.Create(outPath)
	must(err)
	defer f.Close()

	fmt.Fprintf(f, "name: %s\ntopology:\n  nodes:\n", lab.Name)
	for n, nd := range lab.Topology.Nodes {
		fmt.Fprintf(f, "    %s:\n      kind: %s\n      image: %s\n", n, nd.Kind, nd.Image)
	}

	links := collectLinks(devs)
	sort.Slice(links, func(i, j int) bool {
		return links[i].Endpoints[0]+links[i].Endpoints[1] < links[j].Endpoints[0]+links[j].Endpoints[1]
	})
	fmt.Fprintf(f, "  links:\n")
	for _, l := range links {
		fmt.Fprintf(f, "    - endpoints: [\"%s\", \"%s\"]\n", l.Endpoints[0], l.Endpoints[1])
	}
	fmt.Printf("[+] generated %s\n", outPath)
}

// collectLinks collects all valid links between devices for topology.yml.
func collectLinks(devs []device.Device) []topoLink {
	seen := map[string]struct{}{}
	var links []topoLink
	for _, d := range devs {
		for _, m := range d.GetIfMaps() {
			if m.PeerNode == "" {
				continue
			}
			peerDev := findDeviceByHostname(devs, m.PeerNode)
			if peerDev == nil {
				continue
			}
			peerClab := findClabName(peerDev, m.PeerIf)
			if peerClab == "" {
				continue
			}
			ep1 := d.GetNodeName() + ":" + m.Clab
			ep2 := peerDev.GetNodeName() + ":" + peerClab
			key := makeLinkKey(ep1, ep2)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			links = append(links, topoLink{Endpoints: []string{ep1, ep2}})
		}
	}
	return links
}

// findDeviceByHostname returns a device object by its hostname (case-insensitive).
func findDeviceByHostname(devs []device.Device, hostname string) device.Device {
	for _, d := range devs {
		if strings.EqualFold(d.GetHostname(), hostname) {
			return d
		}
	}
	return nil
}

// findClabName returns the clab interface name for a given interface (by clab or src name).
func findClabName(dev device.Device, ifName string) string {
	for _, m := range dev.GetIfMaps() {
		if m.Clab == ifName || m.Src == ifName {
			return m.Clab
		}
	}
	return ""
}

// makeLinkKey returns a consistent key for a pair of endpoints.
func makeLinkKey(a, b string) string {
	if a > b {
		return b + "|" + a
	}
	return a + "|" + b
}

// must panics if err is non-nil (for early bailout).
func must(err error) {
	if err != nil {
		panic(err)
	}
}

// debugDump prints all device states (for development).
func debugDump(devs []device.Device) {
	fmt.Println("=========== DEVICE SLICE DUMP ===========")
	for _, d := range devs {
		fmt.Print(d.Dump())
	}
	fmt.Println("=========================================")
}
