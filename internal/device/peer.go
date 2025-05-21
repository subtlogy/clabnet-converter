package device

import "strings"

// ResolvePeers verifies that each peer node/interface reference is valid.
// If a peer is not valid, its PeerNode/PeerIf fields are cleared.
func ResolvePeers(devs []Device) {
	host2dev := make(map[string]Device, len(devs))
	for _, d := range devs {
		if h := strings.ToLower(d.GetHostname()); h != "" {
			host2dev[h] = d
		}
	}
	for _, d := range devs {
		for i, m := range d.GetIfMaps() {
			if m.PeerNode == "" {
				continue
			}
			pDev, ok := host2dev[strings.ToLower(m.PeerNode)]
			if !ok || !interfaceExists(pDev, m.PeerIf) {
				d.GetIfMaps()[i].PeerNode = ""
				d.GetIfMaps()[i].PeerIf = ""
			}
		}
	}
}

// interfaceExists checks whether a specific interface exists on a device.
func interfaceExists(dev Device, ifName string) bool {
	for _, m := range dev.GetIfMaps() {
		if m.Clab == ifName || m.Src == ifName {
			return true
		}
	}
	return false
}
