package device

// Device defines the common interface for all network devices.
type Device interface {
	GetNodeName() string
	GetKind() string
	GetHostname() string
	GetIfMaps() []IfMap
	RewritePhys([]Device)
	CleanConfig() string
	Dump() string
}
