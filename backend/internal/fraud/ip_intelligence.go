// Package fraud provides anti-fraud detection for the Proxy Coin network.
package fraud

import (
	"context"
	"fmt"
	"net"
	"strings"
)

// IPCategory classifies how an IP address is being used.
type IPCategory string

const (
	IPCategoryResidential IPCategory = "residential"
	IPCategoryDatacenter  IPCategory = "datacenter"
	IPCategoryVPN         IPCategory = "vpn"
	IPCategoryProxy       IPCategory = "proxy"
	IPCategoryUnknown     IPCategory = "unknown"
)

// IPClassification is the result of classifying an IP address.
type IPClassification struct {
	IP          string
	Category    IPCategory
	Country     string
	Region      string
	ISP         string
	ASN         int
	IsResidential bool
}

// IPIntelligence classifies IP addresses as datacenter, residential, or blocked.
// For production, integrate MaxMind GeoIP2 or ipinfo.io.
type IPIntelligence struct {
	// blockedCIDRs holds known datacenter/VPN/Tor CIDR blocks in string prefix form.
	blockedCIDRs []string
	// blockedNets holds parsed CIDR blocks for proper prefix matching.
	blockedNets []*net.IPNet
}

// NewIPIntelligence creates an IPIntelligence checker with the given block list.
// Each entry in blockedCIDRs can be a CIDR (e.g. "10.0.0.0/8") or a prefix
// string (e.g. "192.168.") for simple matching.
func NewIPIntelligence(blockedCIDRs []string) *IPIntelligence {
	ii := &IPIntelligence{blockedCIDRs: blockedCIDRs}
	for _, cidr := range blockedCIDRs {
		if strings.Contains(cidr, "/") {
			if _, network, err := net.ParseCIDR(cidr); err == nil {
				ii.blockedNets = append(ii.blockedNets, network)
			}
		}
	}
	return ii
}

// Check evaluates a single IP address and returns the appropriate verdict.
// Nodes must originate from residential IPs to earn rewards.
func (i *IPIntelligence) Check(ip string) (Verdict, string) {
	parsed := net.ParseIP(ip)

	// Check parsed CIDR networks first (accurate).
	if parsed != nil {
		for _, network := range i.blockedNets {
			if network.Contains(parsed) {
				return VerdictBlock, fmt.Sprintf("ip_intelligence: blocked CIDR %s contains %s", network, ip)
			}
		}
	}

	// Fallback: check string prefixes for simple patterns.
	for _, cidr := range i.blockedCIDRs {
		if !strings.Contains(cidr, "/") && strings.HasPrefix(ip, cidr) {
			return VerdictBlock, "ip_intelligence: datacenter/VPN IP detected: " + ip
		}
	}

	return VerdictAllow, ""
}

// ClassifyIP returns a full classification for the given IP address.
// This is a stub that returns residential by default; real implementation
// should query MaxMind GeoIP2 or a similar provider.
//
// TODO(production): replace with MaxMind GeoIP2 lookup:
//
//	db, _ := maxminddb.Open("/var/lib/GeoIP/GeoIP2-ISP.mmdb")
//	var record maxminddb.ISPRecord
//	db.Lookup(net.ParseIP(ip), &record)
func (i *IPIntelligence) ClassifyIP(ip string) (IPClassification, error) {
	verdict, reason := i.Check(ip)
	if verdict == VerdictBlock {
		cat := IPCategoryDatacenter
		if strings.Contains(reason, "VPN") {
			cat = IPCategoryVPN
		}
		return IPClassification{
			IP:            ip,
			Category:      cat,
			IsResidential: false,
		}, nil
	}

	// Default to residential (stub).
	return IPClassification{
		IP:            ip,
		Category:      IPCategoryResidential,
		IsResidential: true,
	}, nil
}

// VerifyLocation cross-references the claimed country against the GeoIP result.
// Returns true if the IP is consistent with the claimed country.
// Stub implementation: always returns true (MaxMind integration pending).
func (i *IPIntelligence) VerifyLocation(claimedCountry, ip string) (bool, error) {
	classification, err := i.ClassifyIP(ip)
	if err != nil {
		return false, fmt.Errorf("ip_intelligence: classify for location verify: %w", err)
	}

	// If classification has a country, compare. Otherwise pass through.
	if classification.Country == "" || claimedCountry == "" {
		return true, nil
	}
	return strings.EqualFold(classification.Country, claimedCountry), nil
}

// AnalyzeNode queries the database for the node's registered IP and runs
// classification. It emits FraudEvents for any issues found.
// This is the async path called by Detector.Analyze.
func (i *IPIntelligence) AnalyzeNode(ctx context.Context, nodeID string) ([]FraudEvent, error) {
	// In production: fetch node IP from the DB and call ClassifyIP.
	// Stub: no events without DB access.
	_ = ctx
	_ = nodeID
	return nil, nil
}

// knownDatacenterPrefixes returns a curated list of well-known datacenter IP
// string prefixes. Used as a starting point before MaxMind is integrated.
// This list is intentionally incomplete; real deployments should use a proper feed.
func knownDatacenterPrefixes() []string {
	return []string{
		// AWS ranges (sample)
		"3.0.", "3.1.", "3.2.", "3.3.", "3.4.", "3.5.",
		"18.130.", "18.144.", "18.185.", "18.204.", "18.217.",
		"34.192.", "34.208.", "34.224.", "34.240.",
		"52.0.", "52.1.", "52.2.", "52.8.", "52.14.", "52.15.",
		// GCP ranges (sample)
		"34.64.", "34.65.", "34.66.", "34.67.", "34.68.", "34.69.",
		"35.184.", "35.185.", "35.186.", "35.187.", "35.188.",
		// Azure ranges (sample)
		"20.0.", "20.1.", "20.2.", "20.3.", "20.4.", "20.5.",
		"40.64.", "40.65.", "40.66.", "40.67.", "40.68.",
		// DigitalOcean (sample)
		"104.131.", "104.236.", "107.170.", "128.199.", "138.197.", "139.59.",
		"165.22.", "167.71.", "174.138.", "188.166.", "206.189.",
	}
}

// NewIPIntelligenceWithDefaults creates an IPIntelligence that pre-loads a
// curated set of datacenter CIDR prefixes as a starting point.
func NewIPIntelligenceWithDefaults() *IPIntelligence {
	return NewIPIntelligence(knownDatacenterPrefixes())
}
