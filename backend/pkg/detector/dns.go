package detector

import (
	"context"
	"fmt"
	"net"
	"time"
)

type DNSChecker struct {
	timeout time.Duration
}

type DNSResult struct {
	Resolvable bool
	IPs        []string
	Error      error
}

func NewDNSChecker() *DNSChecker {
	return &DNSChecker{
		timeout: 5 * time.Second,
	}
}

func (c *DNSChecker) Check(ctx context.Context, domain string) *DNSResult {
	result := &DNSResult{}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resolver := &net.Resolver{}
	ips, err := resolver.LookupHost(ctx, domain)
	if err != nil {
		result.Error = fmt.Errorf("DNS lookup failed: %w", err)
		return result
	}

	if len(ips) == 0 {
		result.Error = fmt.Errorf("no IP addresses found for domain")
		return result
	}

	result.Resolvable = true
	result.IPs = ips
	return result
}
