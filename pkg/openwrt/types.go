package openwrt

const (
	// UCISectionKey is the ProviderSpecific property name used to carry
	// the UCI config section identifier (e.g. "cfg01a2b3") through
	// the external-dns plan calculation.
	UCISectionKey = "uci-section"
)

// DNSRecord represents a DNS record in LuciRPC
type DNSRecord struct {
	Type   string `json:".type" validate:"required"`
	IP     string `json:"ip,omitempty"`
	Name   string `json:"name,omitempty"`
	CName  string `json:"cname,omitempty"`
	Target string `json:"target,omitempty"`
}
