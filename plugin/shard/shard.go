// Package shard implements an sharding plugin.
package shard

import (
	"context"
	"regexp"
	"strconv"

	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

var log = clog.NewWithPlugin("shard")

// sharder shards DNS responses.
type sharder struct {
	*regexp.Regexp
	plugin.Handler
	maxShards int
}

func newSharder(next plugin.Handler) *sharder {
	return &sharder{
		Regexp:    regexp.MustCompile("^([1-9][0-9]*)\\.([1-9][0-9]*)\\.(.+)$"),
		Handler:   next,
		maxShards: 256,
	}
}

// Name implements the plugin.Handler interface.
func (s *sharder) Name() string { return "shard" }

// ServeDNS implements the plugin.Handler interface.
func (s *sharder) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	if state.QClass() != dns.ClassINET || state.QType() != dns.TypeA {
		return plugin.NextOrFailure(s.Name(), s.Handler, ctx, w, r)
	}

	shardedFqdn := s.sharded((state.Name()))
	if shardedFqdn == nil {
		return plugin.NextOrFailure(s.Name(), s.Handler, ctx, w, r)
	}

	log.Infof("shard A record: %v", shardedFqdn)

	req := &dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:                 r.Id,
			Response:           r.Response,
			Opcode:             r.Opcode,
			Authoritative:      false,
			Truncated:          r.Truncated,
			RecursionDesired:   r.RecursionDesired,
			RecursionAvailable: r.RecursionAvailable,
			Zero:               r.Zero,
			AuthenticatedData:  r.AuthenticatedData,
			CheckingDisabled:   r.CheckingDisabled,
			Rcode:              r.Rcode,
		},

		Compress: r.Compress,

		Question: []dns.Question{{
			Qclass: dns.ClassINET,
			Qtype:  dns.TypeA,
			Name:   shardedFqdn.fqdn,
		}},

		Answer: nil,
		Ns:     nil,
		Extra:  nil,
	}

	return plugin.NextOrFailure(s.Name(), s.Handler, ctx, &writer{
		w,
		shardedFqdn,
	}, req)
}

// shard defines a shard
type shardedFqdn struct {
	requestedFqdn, fqdn string
	shard, numShards    int
}

func (s *sharder) sharded(requestedFqdn string) *shardedFqdn {
	match := s.Regexp.FindStringSubmatch(requestedFqdn)
	if match == nil {
		return nil
	}

	shard, err := strconv.Atoi(match[1])
	if err != nil {
		return nil
	}

	numShards, err := strconv.Atoi(match[2])
	if err != nil {
		return nil
	}

	if shard < 1 || numShards > s.maxShards || shard > numShards {
		return nil
	}

	fqdn := match[3]
	if _, ok := dns.IsDomainName(fqdn); !ok || !dns.IsFqdn(fqdn) {
		return nil
	}

	return &shardedFqdn{requestedFqdn, fqdn, shard, numShards}
}

func (s *shardedFqdn) contains(i int) bool {
	return i%s.numShards == s.shard%s.numShards
}
