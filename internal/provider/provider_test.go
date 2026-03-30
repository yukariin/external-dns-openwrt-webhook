package provider

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/yukariin/external-dns-openwrt-webhook/pkg/logger"
	"github.com/yukariin/external-dns-openwrt-webhook/pkg/openwrt"
	"sigs.k8s.io/external-dns/endpoint"
)

func TestProvider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Provider Suite")
	defer GinkgoRecover()
}

var _ = BeforeSuite(func() {
	if err := logger.Init(&logger.Config{
		Level:    "debug",
		Encoding: "console",
	}); err != nil {
		panic(err)
	}
})

var _ = AfterSuite(func() {
	_ = logger.Log.Sync()
})

var _ = Describe("Provider Suite", func() {
	Context("endpoints records", func() {
		It("should be converted to dns records", func() {
			records := []struct {
				Name   string `json:"name"`
				Type   string `json:"type"`
				Target string `json:"target"`
			}{
				{
					Name:   "a.foobar.com",
					Type:   "A",
					Target: "1.1.1.1",
				},
				{
					Name:   "b.foobar.com",
					Type:   "CNAME",
					Target: "c.foobar.com",
				},
			}

			var endpoints []*endpoint.Endpoint
			for _, record := range records {
				endpoints = append(endpoints, &endpoint.Endpoint{
					DNSName:    record.Name,
					RecordTTL:  defaultTTL,
					RecordType: record.Type,
					Targets:    []string{record.Target},
				})
			}
			dnsRecords := endpoints2DNSRecords(endpoints)
			for index, dnsRecord := range dnsRecords {
				switch dnsRecord.Type {
				case "A":
					Expect(dnsRecord.Name).To(Equal(records[index].Name))
					Expect(dnsRecord.IP).To(Equal(records[index].Target))
				case "CNAME":
					Expect(dnsRecord.CName).To(Equal(records[index].Name))
					Expect(dnsRecord.Target).To(Equal(records[index].Target))
				}
			}
		})

		It("should expand multi-target A records", func() {
			ep := &endpoint.Endpoint{
				DNSName:    "multi.foobar.com",
				RecordType: endpoint.RecordTypeA,
				Targets:    endpoint.Targets{"1.1.1.1", "2.2.2.2"},
			}

			dnsRecords := endpoints2DNSRecords([]*endpoint.Endpoint{ep})
			Expect(dnsRecords).To(HaveLen(2))
			Expect(dnsRecords[0]).To(Equal(openwrt.DNSRecord{Type: "A", Name: "multi.foobar.com", IP: "1.1.1.1"}))
			Expect(dnsRecords[1]).To(Equal(openwrt.DNSRecord{Type: "A", Name: "multi.foobar.com", IP: "2.2.2.2"}))
		})

		It("should only use first target for CNAME", func() {
			ep := &endpoint.Endpoint{
				DNSName:    "alias.foobar.com",
				RecordType: endpoint.RecordTypeCNAME,
				Targets:    endpoint.Targets{"a.foobar.com", "b.foobar.com"},
			}

			dnsRecords := endpoints2DNSRecords([]*endpoint.Endpoint{ep})
			Expect(dnsRecords).To(HaveLen(1))
			Expect(dnsRecords[0]).To(Equal(openwrt.DNSRecord{Type: "CNAME", CName: "alias.foobar.com", Target: "a.foobar.com"}))
		})

		It("dns records to endpoint with uci section key", func() {
			dnsRecords := map[string]openwrt.DNSRecord{
				"cfg01a2b3": {
					Name: "a.foobar.com",
					Type: "A",
					IP:   "1.1.1.1",
				},
				"cfg04d5e6": {
					Type:   "CNAME",
					Target: "c.foobar.com",
					CName:  "b.foobar.com",
				},
			}

			endpoints := dnsRecords2Endpoints(dnsRecords)
			Expect(len(endpoints)).To(Equal(2))

			for _, ep := range endpoints {
				Expect(ep.ProviderSpecific).To(HaveLen(1))
				Expect(ep.ProviderSpecific[0].Name).To(Equal(openwrt.UCISectionKey))
				Expect(ep.ProviderSpecific[0].Value).To(Or(Equal("cfg01a2b3"), Equal("cfg04d5e6")))

				switch ep.RecordType {
				case endpoint.RecordTypeA:
					Expect(ep.DNSName).To(Equal("a.foobar.com"))
					Expect(ep.Targets[0]).To(Equal("1.1.1.1"))
				case endpoint.RecordTypeCNAME:
					Expect(ep.DNSName).To(Equal("b.foobar.com"))
					Expect(ep.Targets[0]).To(Equal("c.foobar.com"))
				}
			}
		})
	})
})
