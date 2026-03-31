package openwrt

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mocks "github.com/yukariin/external-dns-openwrt-webhook/internal/mocks/lucirpc"
	"github.com/yukariin/external-dns-openwrt-webhook/pkg/logger"
	"go.uber.org/mock/gomock"
)

func TestOpenWRT(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OpenWRT Suite")
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

var _ = Describe("OpenWRT", func() {
	var (
		ctx         context.Context
		mockCtrl    *gomock.Controller
		mockLuciRPC *mocks.MockLuciRPC
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockCtrl = gomock.NewController(GinkgoT())
		mockLuciRPC = mocks.NewMockLuciRPC(mockCtrl)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("Get DNS", func() {
		It("returns error when Uci fails", func() {
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return("", fmt.Errorf("connection refused"))

			o := openWRT{lucirpc: mockLuciRPC}
			records, err := o.GetDNSRecords(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("connection refused"))
			Expect(records).To(BeNil())
		})

		It("returns empty map when result is empty", func() {
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return("", nil)

			o := openWRT{lucirpc: mockLuciRPC}
			records, err := o.GetDNSRecords(ctx)
			Expect(err).To(BeNil())
			Expect(records).To(BeEmpty())
		})

		It("get all records", func() {
			expectedJson, err := json.Marshal(map[string]DNSRecord{
				"x": {
					Type: "domain",
					Name: "foobar",
					IP:   "1.1.1.1",
				},
				"y": {
					Type:   "cname",
					CName:  "foobar",
					Target: "bar.foo.com",
				},
				"z": {
					Type: "whatever",
				},
				"t": {
					Type:  "txt",
					Name:  "k8s.example",
					Value: "heritage=external-dns",
				},
			})
			Expect(err).To(BeNil())
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(string(expectedJson), nil)
			o := openWRT{
				lucirpc: mockLuciRPC,
			}
			resultDNS, err := o.GetDNSRecords(ctx)
			Expect(err).To(BeNil())
			Expect(resultDNS).ToNot(BeNil())
			Expect(resultDNS).To(Equal(map[string]DNSRecord{
				"x": {
					Type: "A",
					Name: "foobar",
					IP:   "1.1.1.1",
				},
				"y": {
					Type:   "CNAME",
					CName:  "foobar",
					Target: "bar.foo.com",
				},
				"t": {
					Type:  "TXT",
					Name:  "k8s.example",
					Value: "heritage=external-dns",
				},
			}))
		})
	})

	Context("Set DNS", func() {
		It("set A record with success", func() {
			cfg := "foobar"
			ip := "1.1.1.1"
			name := "foo.bar.com"

			mockLuciRPC.EXPECT().Uci(ctx, "add", []string{"dhcp", "domain"}).Return(cfg, nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "name", name}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "ip", ip}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "commit", []string{"dhcp"}).Return("", nil)

			o := openWRT{
				lucirpc: mockLuciRPC,
			}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{
					Type: "A",
					IP:   ip,
					Name: name,
				},
			})
			Expect(err).To(BeNil())
		})

		It("A without name", func() {
			o := openWRT{}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{
					Type: "A",
					IP:   "1.1.1.1",
				},
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("name is required"))
		})

		It("A without ip", func() {
			o := openWRT{}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{
					Type: "A",
					Name: "foobar",
				},
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("ip is required"))
		})

		It("set CNAME record", func() {
			cfg := "foobar"
			cname := "foo.bar.com"
			target := "bar.foo.com"

			mockLuciRPC.EXPECT().Uci(ctx, "add", []string{"dhcp", "cname"}).Return(cfg, nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "cname", cname}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "target", target}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "commit", []string{"dhcp"}).Return("", nil)

			o := openWRT{
				lucirpc: mockLuciRPC,
			}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{
					Type:   "CNAME",
					CName:  cname,
					Target: target,
				},
			})
			Expect(err).To(BeNil())
		})

		It("CNAME without cname", func() {
			o := openWRT{}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{
					Type:   "CNAME",
					Target: "foo.bar.com",
				},
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("cname is required"))
		})

		It("CNAME without target", func() {
			o := openWRT{}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{
					Type:  "CNAME",
					CName: "foobar",
				},
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("target is required"))
		})

		It("set TXT record", func() {
			cfg := "foobar"
			name := "k8s.test"
			value := "heritage=external-dns,external-dns/owner=k8s"

			mockLuciRPC.EXPECT().Uci(ctx, "add", []string{"dhcp", "txt"}).Return(cfg, nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "name", name}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "value", value}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "commit", []string{"dhcp"}).Return("", nil)

			o := openWRT{
				lucirpc: mockLuciRPC,
			}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{
					Type:  "TXT",
					Name:  name,
					Value: value,
				},
			})
			Expect(err).To(BeNil())
		})

		It("TXT without name", func() {
			o := openWRT{}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{
					Type:  "TXT",
					Value: "heritage=external-dns",
				},
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("name is required"))
		})

		It("TXT without value", func() {
			o := openWRT{}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{
					Type: "TXT",
					Name: "k8s.test",
				},
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("value is required"))
		})

		It("rejects invalid record type", func() {
			o := openWRT{}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{Type: "MX", Name: "foo.bar.com"},
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("invalid record type: MX"))
		})

		It("sets multiple records with single commit", func() {
			mockLuciRPC.EXPECT().Uci(ctx, "add", []string{"dhcp", "domain"}).Return("cfg01", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", "cfg01", "name", "a.example.com"}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", "cfg01", "ip", "1.1.1.1"}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "add", []string{"dhcp", "cname"}).Return("cfg02", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", "cfg02", "cname", "b.example.com"}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", "cfg02", "target", "a.example.com"}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "commit", []string{"dhcp"}).Return("", nil)

			o := openWRT{lucirpc: mockLuciRPC}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{Type: "A", Name: "a.example.com", IP: "1.1.1.1"},
				{Type: "CNAME", CName: "b.example.com", Target: "a.example.com"},
			})
			Expect(err).To(BeNil())
		})
	})

	Context("Delete DNS", func() {
		It("returns error when GetDNSRecords fails", func() {
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return("", fmt.Errorf("timeout"))

			o := openWRT{lucirpc: mockLuciRPC}
			err := o.DeleteDNSRecords(ctx, []DNSRecord{
				{Type: "A", Name: "foo.com", IP: "1.1.1.1"},
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("timeout"))
		})

		It("returns error when delete RPC call fails", func() {
			currentRecords := map[string]DNSRecord{
				"x": {Type: "domain", Name: "foo.com", IP: "1.1.1.1"},
			}
			currentJson, err := json.Marshal(currentRecords)
			Expect(err).To(BeNil())

			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(string(currentJson), nil)
			mockLuciRPC.EXPECT().Uci(ctx, "delete", []string{"dhcp", "x"}).Return("", fmt.Errorf("permission denied"))

			o := openWRT{lucirpc: mockLuciRPC}
			err = o.DeleteDNSRecords(ctx, []DNSRecord{
				{Type: "A", Name: "foo.com", IP: "1.1.1.1"},
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("permission denied"))
		})

		It("delete A record", func() {
			cfg := "x"
			name := "happy.com"
			ip := "2.2.2.2"

			expectedCurrentDNSRecords := map[string]DNSRecord{
				cfg: {
					Type: "domain",
					Name: name,
					IP:   ip,
				},
				"y": {
					Type:   "cname",
					CName:  "foo.bar.com",
					Target: "bar.foo.com",
				},
			}

			expectedCurrentJson, err := json.Marshal(expectedCurrentDNSRecords)
			Expect(err).To(BeNil())
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(string(expectedCurrentJson), nil)
			mockLuciRPC.EXPECT().Uci(ctx, "delete", []string{"dhcp", cfg}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "commit", []string{"dhcp"}).Return("", nil)

			o := openWRT{
				lucirpc: mockLuciRPC,
			}
			err = o.DeleteDNSRecords(ctx, []DNSRecord{
				{
					Type: "A",
					Name: name,
					IP:   ip,
				},
			})
			Expect(err).To(BeNil())
		})

		It("delete CNAME record", func() {
			cfg := "y"
			cname := "happy.com"
			target := "foo.bar.com"

			expectedCurrentDNSRecords := map[string]DNSRecord{
				"x": {
					Type: "domain",
					Name: "happy.com",
					IP:   "1.1.1.1",
				},
				cfg: {
					Type:   "cname",
					CName:  cname,
					Target: target,
				},
			}

			expectedCurrentJson, err := json.Marshal(expectedCurrentDNSRecords)
			Expect(err).To(BeNil())
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(string(expectedCurrentJson), nil)
			mockLuciRPC.EXPECT().Uci(ctx, "delete", []string{"dhcp", cfg}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "commit", []string{"dhcp"}).Return("", nil)

			o := openWRT{
				lucirpc: mockLuciRPC,
			}
			err = o.DeleteDNSRecords(ctx, []DNSRecord{
				{
					Type:   "CNAME",
					CName:  cname,
					Target: target,
				},
			})
			Expect(err).To(BeNil())
		})

		It("not found", func() {
			expectedCurrentDNSRecords := map[string]DNSRecord{
				"x": {
					Type: "domain",
					Name: "happy.com",
					IP:   "1.1.1.1",
				},
				"y": {
					Type:   "cname",
					CName:  "foo.bar.com",
					Target: "bar.foo.com",
				},
			}

			expectedCurrentJson, err := json.Marshal(expectedCurrentDNSRecords)
			Expect(err).To(BeNil())
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(string(expectedCurrentJson), nil)

			o := openWRT{
				lucirpc: mockLuciRPC,
			}
			err = o.DeleteDNSRecords(ctx, []DNSRecord{
				{
					Type:   "CNAME",
					CName:  "whatever",
					Target: "3.3.3.3",
				},
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("records not found for deletion"))
		})

		It("delete A record with duplicate names deletes only matching IP", func() {
			// Two A records for "happy.com" with different IPs — round-robin scenario
			cfg1 := "x"
			cfg2 := "w"
			name := "happy.com"

			expectedCurrentDNSRecords := map[string]DNSRecord{
				cfg1: {
					Type: "domain",
					Name: name,
					IP:   "1.1.1.1",
				},
				cfg2: {
					Type: "domain",
					Name: name,
					IP:   "2.2.2.2",
				},
			}

			expectedCurrentJson, err := json.Marshal(expectedCurrentDNSRecords)
			Expect(err).To(BeNil())
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(string(expectedCurrentJson), nil)
			// Only cfg2 should be deleted (matches name + IP)
			mockLuciRPC.EXPECT().Uci(ctx, "delete", []string{"dhcp", cfg2}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "commit", []string{"dhcp"}).Return("", nil)

			o := openWRT{
				lucirpc: mockLuciRPC,
			}
			err = o.DeleteDNSRecords(ctx, []DNSRecord{
				{
					Type: "A",
					Name: name,
					IP:   "2.2.2.2",
				},
			})
			Expect(err).To(BeNil())
		})

		It("delete A record with wrong IP does not match", func() {
			expectedCurrentDNSRecords := map[string]DNSRecord{
				"x": {
					Type: "domain",
					Name: "happy.com",
					IP:   "1.1.1.1",
				},
			}

			expectedCurrentJson, err := json.Marshal(expectedCurrentDNSRecords)
			Expect(err).To(BeNil())
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(string(expectedCurrentJson), nil)

			o := openWRT{
				lucirpc: mockLuciRPC,
			}
			err = o.DeleteDNSRecords(ctx, []DNSRecord{
				{
					Type: "A",
					Name: "happy.com",
					IP:   "9.9.9.9", // wrong IP — should not match
				},
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("records not found for deletion"))
		})

		It("delete TXT record", func() {
			cfg := "t"
			name := "k8s.test.example.org"
			value := "heritage=external-dns,external-dns/owner=k8s"

			expectedCurrentDNSRecords := map[string]DNSRecord{
				"x": {
					Type: "domain",
					Name: "happy.com",
					IP:   "1.1.1.1",
				},
				cfg: {
					Type:  "txt",
					Name:  name,
					Value: value,
				},
			}

			expectedCurrentJson, err := json.Marshal(expectedCurrentDNSRecords)
			Expect(err).To(BeNil())
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(string(expectedCurrentJson), nil)
			mockLuciRPC.EXPECT().Uci(ctx, "delete", []string{"dhcp", cfg}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "commit", []string{"dhcp"}).Return("", nil)

			o := openWRT{
				lucirpc: mockLuciRPC,
			}
			err = o.DeleteDNSRecords(ctx, []DNSRecord{
				{
					Type:  "TXT",
					Name:  name,
					Value: value,
				},
			})
			Expect(err).To(BeNil())
		})

		It("delete both A records and TXT together", func() {
			cfg1 := "x"
			cfg2 := "w"
			cfgTxt := "t"
			name := "test.example.org"

			expectedCurrentDNSRecords := map[string]DNSRecord{
				cfg1: {
					Type: "domain",
					Name: name,
					IP:   "1.1.1.1",
				},
				cfg2: {
					Type: "domain",
					Name: name,
					IP:   "2.2.2.2",
				},
				cfgTxt: {
					Type:  "txt",
					Name:  "k8s." + name,
					Value: "heritage=external-dns,external-dns/owner=k8s",
				},
			}

			expectedCurrentJson, err := json.Marshal(expectedCurrentDNSRecords)
			Expect(err).To(BeNil())
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(string(expectedCurrentJson), nil)
			mockLuciRPC.EXPECT().Uci(ctx, "delete", []string{"dhcp", cfg1}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "delete", []string{"dhcp", cfg2}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "delete", []string{"dhcp", cfgTxt}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "commit", []string{"dhcp"}).Return("", nil)

			o := openWRT{
				lucirpc: mockLuciRPC,
			}
			err = o.DeleteDNSRecords(ctx, []DNSRecord{
				{Type: "A", Name: name, IP: "1.1.1.1"},
				{Type: "A", Name: name, IP: "2.2.2.2"},
				{Type: "TXT", Name: "k8s." + name, Value: "heritage=external-dns,external-dns/owner=k8s"},
			})
			Expect(err).To(BeNil())
		})
	})

	Context("recordMatches", func() {
		It("matches A record by type name and ip", func() {
			Expect(recordMatches(DNSRecord{Type: "A", Name: "foo.com", IP: "1.1.1.1"}, DNSRecord{Type: "A", Name: "foo.com", IP: "1.1.1.1"})).To(BeTrue())
			Expect(recordMatches(DNSRecord{Type: "A", Name: "foo.com", IP: "1.1.1.1"}, DNSRecord{Type: "A", Name: "foo.com", IP: "2.2.2.2"})).To(BeFalse())
			Expect(recordMatches(DNSRecord{Type: "A", Name: "foo.com", IP: "1.1.1.1"}, DNSRecord{Type: "A", Name: "bar.com", IP: "1.1.1.1"})).To(BeFalse())
		})

		It("matches CNAME record by type cname and target", func() {
			Expect(recordMatches(DNSRecord{Type: "CNAME", CName: "foo.com", Target: "bar.com"}, DNSRecord{Type: "CNAME", CName: "foo.com", Target: "bar.com"})).To(BeTrue())
			Expect(recordMatches(DNSRecord{Type: "CNAME", CName: "foo.com", Target: "bar.com"}, DNSRecord{Type: "CNAME", CName: "foo.com", Target: "baz.com"})).To(BeFalse())
		})

		It("does not match across types", func() {
			Expect(recordMatches(DNSRecord{Type: "A", Name: "foo.com", IP: "1.1.1.1"}, DNSRecord{Type: "CNAME", CName: "foo.com", Target: "1.1.1.1"})).To(BeFalse())
		})

		It("matches TXT record by type name and value", func() {
			Expect(recordMatches(DNSRecord{Type: "TXT", Name: "k8s.foo", Value: "heritage=external-dns"}, DNSRecord{Type: "TXT", Name: "k8s.foo", Value: "heritage=external-dns"})).To(BeTrue())
			Expect(recordMatches(DNSRecord{Type: "TXT", Name: "k8s.foo", Value: "heritage=external-dns"}, DNSRecord{Type: "TXT", Name: "k8s.foo", Value: "other"})).To(BeFalse())
			Expect(recordMatches(DNSRecord{Type: "TXT", Name: "k8s.foo", Value: "heritage=external-dns"}, DNSRecord{Type: "TXT", Name: "k8s.bar", Value: "heritage=external-dns"})).To(BeFalse())
		})
	})
})
