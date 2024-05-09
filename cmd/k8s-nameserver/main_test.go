// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause

//go:build !plan9

package main

import (
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/miekg/dns"
	"tailscale.com/util/dnsname"
)

func TestNameserver(t *testing.T) {

	tests := []struct {
		name     string
		ip4      map[dnsname.FQDN][]net.IP
		query    *dns.Msg
		wantResp *dns.Msg
	}{
		{
			name: "A record query, record exists",
			ip4:  map[dnsname.FQDN][]net.IP{dnsname.FQDN("foo.bar.com."): {{1, 2, 3, 4}}},
			query: &dns.Msg{
				Question: []dns.Question{{Name: "foo.bar.com", Qtype: dns.TypeA}},
				MsgHdr:   dns.MsgHdr{Id: 1, RecursionDesired: true},
			},
			wantResp: &dns.Msg{
				Answer: []dns.RR{&dns.A{Hdr: dns.RR_Header{
					Name: "foo.bar.com", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 0},
					A: net.IP{1, 2, 3, 4}}},
				Question: []dns.Question{{Name: "foo.bar.com", Qtype: dns.TypeA}},
				MsgHdr: dns.MsgHdr{
					Id:                 1,
					Rcode:              dns.RcodeSuccess,
					RecursionAvailable: false,
					RecursionDesired:   true,
					Response:           true,
					Opcode:             dns.OpcodeQuery,
					Authoritative:      true,
				}},
		},
		{
			name: "A record query, record does not exist",
			ip4:  map[dnsname.FQDN][]net.IP{dnsname.FQDN("foo.bar.com."): {{1, 2, 3, 4}}},
			query: &dns.Msg{
				Question: []dns.Question{{Name: "baz.bar.com", Qtype: dns.TypeA}},
				MsgHdr:   dns.MsgHdr{Id: 1},
			},
			wantResp: &dns.Msg{
				Question: []dns.Question{{Name: "baz.bar.com", Qtype: dns.TypeA}},
				MsgHdr: dns.MsgHdr{
					Id:                 1,
					Rcode:              dns.RcodeNameError,
					RecursionAvailable: false,
					Response:           true,
					Opcode:             dns.OpcodeQuery,
					Authoritative:      true,
				}},
		},
		{
			name: "A record query, but the name is not a valid FQDN",
			ip4:  map[dnsname.FQDN][]net.IP{dnsname.FQDN("foo.bar.com."): {{1, 2, 3, 4}}},
			query: &dns.Msg{
				Question: []dns.Question{{Name: "foo..bar.com", Qtype: dns.TypeA}},
				MsgHdr:   dns.MsgHdr{Id: 1},
			},
			wantResp: &dns.Msg{
				Question: []dns.Question{{Name: "foo..bar.com", Qtype: dns.TypeA}},
				MsgHdr: dns.MsgHdr{
					Id:       1,
					Rcode:    dns.RcodeFormatError,
					Response: true,
					Opcode:   dns.OpcodeQuery,
				}},
		},
		{
			name: "AAAA record query",
			ip4:  map[dnsname.FQDN][]net.IP{dnsname.FQDN("foo.bar.com."): {{1, 2, 3, 4}}},
			query: &dns.Msg{
				Question: []dns.Question{{Name: "foo.bar.com", Qtype: dns.TypeAAAA}},
				MsgHdr:   dns.MsgHdr{Id: 1},
			},
			wantResp: &dns.Msg{
				Question: []dns.Question{{Name: "foo.bar.com", Qtype: dns.TypeAAAA}},
				MsgHdr: dns.MsgHdr{
					Id:       1,
					Rcode:    dns.RcodeNotImplemented,
					Response: true,
					Opcode:   dns.OpcodeQuery,
				}},
		},
		{
			name: "AAAA record query",
			ip4:  map[dnsname.FQDN][]net.IP{dnsname.FQDN("foo.bar.com."): {{1, 2, 3, 4}}},
			query: &dns.Msg{
				Question: []dns.Question{{Name: "foo.bar.com", Qtype: dns.TypeAAAA}},
				MsgHdr:   dns.MsgHdr{Id: 1},
			},
			wantResp: &dns.Msg{
				Question: []dns.Question{{Name: "foo.bar.com", Qtype: dns.TypeAAAA}},
				MsgHdr: dns.MsgHdr{
					Id:       1,
					Rcode:    dns.RcodeNotImplemented,
					Response: true,
					Opcode:   dns.OpcodeQuery,
				}},
		},
		{
			name: "CNAME record query",
			ip4:  map[dnsname.FQDN][]net.IP{dnsname.FQDN("foo.bar.com."): {{1, 2, 3, 4}}},
			query: &dns.Msg{
				Question: []dns.Question{{Name: "foo.bar.com", Qtype: dns.TypeCNAME}},
				MsgHdr:   dns.MsgHdr{Id: 1},
			},
			wantResp: &dns.Msg{
				Question: []dns.Question{{Name: "foo.bar.com", Qtype: dns.TypeCNAME}},
				MsgHdr: dns.MsgHdr{
					Id:       1,
					Rcode:    dns.RcodeNotImplemented,
					Response: true,
					Opcode:   dns.OpcodeQuery,
				}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := &nameserver{
				ip4: tt.ip4,
			}
			handler := ns.handleFunc()
			fakeRespW := &fakeResponseWriter{}
			handler(fakeRespW, tt.query)
			if diff := cmp.Diff(*fakeRespW.msg, *tt.wantResp); diff != "" {
				t.Fatalf("unexpected response (-got +want): \n%s", diff)
			}
		})
	}
}

func TestResetRecords(t *testing.T) {
	tests := []struct {
		name     string
		config   []byte
		hasIp4   map[dnsname.FQDN][]net.IP
		wantsIp4 map[dnsname.FQDN][]net.IP
		wantsErr bool
	}{
		{
			name:     "previously empty nameserver.ip4 gets set",
			config:   []byte(`{"version": "v1alpha1", "ip4": {"foo.bar.com": ["1.2.3.4"]}}`),
			wantsIp4: map[dnsname.FQDN][]net.IP{"foo.bar.com.": {{1, 2, 3, 4}}},
		},
		{
			name:     "nameserver.ip4 gets reset",
			hasIp4:   map[dnsname.FQDN][]net.IP{"baz.bar.com.": {{1, 1, 3, 3}}},
			config:   []byte(`{"version": "v1alpha1", "ip4": {"foo.bar.com": ["1.2.3.4"]}}`),
			wantsIp4: map[dnsname.FQDN][]net.IP{"foo.bar.com.": {{1, 2, 3, 4}}},
		},
		{
			name:     "configuration with incompatible version",
			hasIp4:   map[dnsname.FQDN][]net.IP{"baz.bar.com.": {{1, 1, 3, 3}}},
			config:   []byte(`{"version": "v1beta1", "ip4": {"foo.bar.com": ["1.2.3.4"]}}`),
			wantsIp4: map[dnsname.FQDN][]net.IP{"baz.bar.com.": {{1, 1, 3, 3}}},
			wantsErr: true,
		},
		{
			name:     "nameserver.ip4 gets reset to empty config when no configuration is provided",
			hasIp4:   map[dnsname.FQDN][]net.IP{"baz.bar.com.": {{1, 1, 3, 3}}},
			wantsIp4: make(map[dnsname.FQDN][]net.IP),
		},
		{
			name:     "nameserver.ip4 gets reset to empty config when the provided configuration is empty",
			hasIp4:   map[dnsname.FQDN][]net.IP{"baz.bar.com.": {{1, 1, 3, 3}}},
			config:   []byte(`{"version": "v1alpha1", "ip4": {}}`),
			wantsIp4: make(map[dnsname.FQDN][]net.IP),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := &nameserver{
				ip4:          tt.hasIp4,
				configReader: func() ([]byte, error) { return tt.config, nil },
			}
			if err := ns.resetRecords(); err == nil == tt.wantsErr {
				t.Errorf("resetRecords() returned err: %v, wantsErr: %v", err, tt.wantsErr)
			}
			if diff := cmp.Diff(ns.ip4, tt.wantsIp4); diff != "" {
				t.Fatalf("unexpected nameserver.ip4 contents (-got +want): \n%s", diff)
			}
		})
	}
}

// fakeResponseWriter is a faked out dns.ResponseWriter that can be used in
// tests that need to read the response message that was written.
type fakeResponseWriter struct {
	msg *dns.Msg
}

var _ dns.ResponseWriter = &fakeResponseWriter{}

func (fr *fakeResponseWriter) WriteMsg(msg *dns.Msg) error {
	fr.msg = msg
	return nil
}
func (fr *fakeResponseWriter) LocalAddr() net.Addr {
	return nil
}
func (fr *fakeResponseWriter) RemoteAddr() net.Addr {
	return nil
}
func (fr *fakeResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}
func (fr *fakeResponseWriter) Close() error {
	return nil
}
func (fr *fakeResponseWriter) TsigStatus() error {
	return nil
}
func (fr *fakeResponseWriter) TsigTimersOnly(bool) {}
func (fr *fakeResponseWriter) Hijack()             {}
