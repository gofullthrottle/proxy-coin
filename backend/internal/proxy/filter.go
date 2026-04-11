// Package proxy handles HTTP/HTTPS proxy request routing and forwarding.
package proxy

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
)

// BlocklistCategory classifies why a domain is blocked.
type BlocklistCategory string

const (
	// CategoryCSAM blocks child sexual abuse material domains.
	CategoryCSAM BlocklistCategory = "csam"
	// CategoryMalware blocks known malware / C2 infrastructure.
	CategoryMalware BlocklistCategory = "malware"
	// CategoryGovernment blocks government-operated domains (.gov, .mil).
	CategoryGovernment BlocklistCategory = "government"
	// CategoryFinancial blocks financial institution domains.
	CategoryFinancial BlocklistCategory = "financial"
	// CategoryHealthcare blocks healthcare provider domains.
	CategoryHealthcare BlocklistCategory = "healthcare"
)

// governmentSuffixes are TLDs and second-level domains reserved for government
// and military use. Traffic to these must never transit through user devices.
var governmentSuffixes = []string{
	".gov", ".mil",
	".gov.uk", ".mod.uk",
	".gc.ca",   // Canadian federal government
	".gouv.fr", // French government
	".bund.de", // German federal government
}

// financialDomains is a seed list of major financial institutions whose
// traffic must not be proxied through third-party devices.
var financialDomains = []string{
	"irs.gov",
	"treasury.gov",
	"federalreserve.gov",
	"fdic.gov",
	"sec.gov",
	"finra.org",
	"swift.com",
	"wellsfargo.com",
	"bankofamerica.com",
	"jpmorgan.com",
	"chase.com",
	"citibank.com",
	"goldmansachs.com",
	"morganstanley.com",
	"barclays.com",
	"hsbc.com",
	"deutschebank.com",
	"bnpparibas.com",
	"creditsuisse.com",
	"ubs.com",
}

// healthcareDomains is a seed list of healthcare organisations whose
// traffic must not be proxied.
var healthcareDomains = []string{
	"cms.gov",
	"medicare.gov",
	"medicaid.gov",
	"hhs.gov",
	"cdc.gov",
	"nih.gov",
	"fda.gov",
	"va.gov",
	"tricare.mil",
}

// DomainFilter maintains a hash-based blocklist for privacy-preserving domain
// filtering. Domains are stored as SHA-256 hashes so the filter can be
// distributed to nodes without leaking the plaintext domain names.
//
// All methods are safe for concurrent use.
type DomainFilter struct {
	mu        sync.RWMutex
	blocklist map[string]BlocklistCategory // SHA-256 hex hash → category
	version   atomic.Int64
}

// NewDomainFilter returns an empty DomainFilter. Call LoadFromSuffixes to
// populate the built-in seed lists.
func NewDomainFilter() *DomainFilter {
	return &DomainFilter{
		blocklist: make(map[string]BlocklistCategory),
	}
}

// hashDomain computes the canonical SHA-256 hash for a domain string.
// The domain is lower-cased and stripped of a leading dot before hashing.
func hashDomain(domain string) string {
	domain = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(domain), "."))
	sum := sha256.Sum256([]byte(domain))
	return hex.EncodeToString(sum[:])
}

// extractDomain parses rawURL (or a bare "host:port" for CONNECT targets) and
// returns the bare hostname without port or scheme.
func extractDomain(rawURL string) string {
	// Fast-path: bare host:port (used for CONNECT requests).
	if !strings.Contains(rawURL, "://") {
		host := rawURL
		if idx := strings.LastIndex(host, ":"); idx != -1 {
			host = host[:idx]
		}
		return strings.ToLower(host)
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return strings.ToLower(rawURL)
	}
	host := u.Hostname()
	return strings.ToLower(host)
}

// IsBlocked reports whether rawURL should be blocked, and if so the category.
func (f *DomainFilter) IsBlocked(rawURL string) (bool, BlocklistCategory) {
	domain := extractDomain(rawURL)
	if domain == "" {
		return false, ""
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	// Check the full domain first, then progressively shorter suffixes to
	// handle sub-domains (e.g. "api.irs.gov" → check "api.irs.gov", "irs.gov", "gov").
	parts := strings.Split(domain, ".")
	for i := 0; i < len(parts); i++ {
		candidate := strings.Join(parts[i:], ".")
		if cat, ok := f.blocklist[hashDomain(candidate)]; ok {
			return true, cat
		}
	}
	return false, ""
}

// AddDomain adds a single domain (e.g. "example.gov") to the blocklist under
// the given category. The domain is stored as its SHA-256 hash.
func (f *DomainFilter) AddDomain(domain string, category BlocklistCategory) {
	h := hashDomain(domain)
	f.mu.Lock()
	f.blocklist[h] = category
	f.mu.Unlock()
	f.version.Add(1)
}

// AddDomains bulk-adds a map of domain → category entries in a single lock
// acquisition, which is more efficient than calling AddDomain in a loop.
func (f *DomainFilter) AddDomains(domains map[string]BlocklistCategory) {
	if len(domains) == 0 {
		return
	}
	f.mu.Lock()
	for domain, cat := range domains {
		f.blocklist[hashDomain(domain)] = cat
	}
	f.mu.Unlock()
	f.version.Add(1)
}

// LoadFromSuffixes populates the blocklist with the built-in seed sets:
//   - government TLD/SLD suffixes (CategoryGovernment)
//   - known financial institution domains (CategoryFinancial)
//   - known healthcare domains (CategoryHealthcare)
//
// This is called once at startup; the operator may also push additional
// entries via AddDomain / AddDomains from an admin API.
func (f *DomainFilter) LoadFromSuffixes() {
	batch := make(map[string]BlocklistCategory,
		len(governmentSuffixes)+len(financialDomains)+len(healthcareDomains))

	for _, s := range governmentSuffixes {
		// Store without leading dot so suffix matching in IsBlocked works.
		clean := strings.TrimPrefix(s, ".")
		batch[clean] = CategoryGovernment
	}
	for _, d := range financialDomains {
		batch[d] = CategoryFinancial
	}
	for _, d := range healthcareDomains {
		batch[d] = CategoryHealthcare
	}

	f.AddDomains(batch)
}

// GetBlocklistHashes returns every SHA-256 hex hash in the blocklist.
// The slice is safe to iterate; it is a snapshot taken under the read lock.
func (f *DomainFilter) GetBlocklistHashes() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	hashes := make([]string, 0, len(f.blocklist))
	for h := range f.blocklist {
		hashes = append(hashes, h)
	}
	return hashes
}

// GetVersion returns the current blocklist version counter. The version
// increments on every write operation and is used by nodes to detect stale
// local copies.
func (f *DomainFilter) GetVersion() int64 {
	return f.version.Load()
}

// SetVersion forcibly sets the version counter. Use this when restoring a
// persisted blocklist so that the version reflects its stored state.
func (f *DomainFilter) SetVersion(v int64) {
	f.version.Store(v)
}

// GetBlocklistForDistribution serialises the current blocklist as a newline-
// delimited list of "<hash>:<category>" pairs, suitable for sending to nodes.
// It also returns the current version number.
//
// The format is intentionally simple (not protobuf) so nodes can store and
// compare it without a schema dependency.
func (f *DomainFilter) GetBlocklistForDistribution() ([]byte, int64) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	version := f.version.Load()

	// Pre-allocate a conservative capacity.
	var sb strings.Builder
	sb.Grow(len(f.blocklist) * 80)

	for h, cat := range f.blocklist {
		sb.WriteString(h)
		sb.WriteByte(':')
		sb.WriteString(string(cat))
		sb.WriteByte('\n')
	}

	return []byte(sb.String()), version
}
