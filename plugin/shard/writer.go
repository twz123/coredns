package shard

import (
	"bytes"
	"sort"

	"github.com/miekg/dns"
)

type writer struct {
	dns.ResponseWriter
	*shardedFqdn
}

// WriteMsg records the status code and calls the
// underlying ResponseWriter's WriteMsg method.
func (w *writer) WriteMsg(res *dns.Msg) error {

	var aRecords aByIP
	for _, rr := range res.Answer {
		if a, ok := rr.(*dns.A); ok && a.Hdr.Name == w.fqdn {
			aRecords = append(aRecords, a)
		}
	}
	sort.Sort(aRecords)

	seen := 0
	var sharded []dns.RR
	for _, a := range aRecords {
		seen = seen + 1
		if !w.shardedFqdn.contains(seen) {
			continue
		}

		shardedA := *a // copy
		shardedA.Hdr.Name = w.requestedFqdn
		sharded = append(sharded, &shardedA)
	}

	return w.ResponseWriter.WriteMsg(&dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:                 res.Id,
			Response:           res.Response,
			Opcode:             res.Opcode,
			Authoritative:      false,
			Truncated:          res.Truncated,
			RecursionDesired:   res.RecursionDesired,
			RecursionAvailable: res.RecursionAvailable,
			Zero:               res.Zero,
			AuthenticatedData:  res.AuthenticatedData,
			CheckingDisabled:   res.CheckingDisabled,
			Rcode:              res.Rcode,
		},

		Compress: res.Compress,

		Question: []dns.Question{{
			Qclass: dns.ClassINET,
			Qtype:  dns.TypeA,
			Name:   w.requestedFqdn,
		}},

		Answer: sharded,
		Ns:     nil,
		Extra:  nil,
	})
}

func (w *writer) Write(buf []byte) (int, error) {
	return w.ResponseWriter.Write(buf)
}

type aByIP []*dns.A

func (s aByIP) Len() int {
	return len(s)
}

func (s aByIP) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s aByIP) Less(i, j int) bool {
	return bytes.Compare(s[i].A, s[j].A) < 0
}
