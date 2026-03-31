package lucirpc

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/yukariin/external-dns-openwrt-webhook/pkg/logger"
)

func TestLuciRPC(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Luci RPC Suite")
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

var _ = Describe("Luci RPC", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Context("auth", func() {
		It("should be login", func() {
			mux := http.NewServeMux()
			ts := httptest.NewServer(mux)
			defer ts.Close()
			u, err := url.Parse(ts.URL)
			Expect(err).To(BeNil())
			port, err := strconv.Atoi(u.Port())
			Expect(err).To(BeNil())
			hostname := u.Hostname()

			config := DefaultConfig()
			Expect(config).ToNot(BeNil())
			config.SSL = false
			config.Hostname = hostname
			config.Port = port

			client := lucirpc{
				config:     config,
				httpClient: ts.Client(),
			}
			Expect(client).ToNot(BeNil())

			mux.HandleFunc(authPath, func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodPost))
				Expect(r.URL.Path).To(Equal(authPath))
				w.WriteHeader(http.StatusAccepted)
				_, err = w.Write([]byte(`{"result":"foobar"}`))
				Expect(err).To(BeNil())
			})

			err = client.auth(ctx)
			Expect(err).To(BeNil())
			Expect(client.token).To(Equal("foobar"))
		})

		It("should be unauthorized", func() {
			mux := http.NewServeMux()
			ts := httptest.NewServer(mux)
			defer ts.Close()
			u, err := url.Parse(ts.URL)
			Expect(err).To(BeNil())
			port, err := strconv.Atoi(u.Port())
			Expect(err).To(BeNil())
			hostname := u.Hostname()

			config := DefaultConfig()
			Expect(config).ToNot(BeNil())
			config.SSL = false
			config.Hostname = hostname
			config.Port = port

			client := lucirpc{
				config:     config,
				httpClient: ts.Client(),
			}
			Expect(client).ToNot(BeNil())

			mux.HandleFunc(authPath, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			})

			err = client.auth(ctx)
			Expect(err).To(Equal(ErrHttpUnauthorized))
			Expect(client.token).To(Equal(""))
		})

		It("should be forbidden", func() {
			mux := http.NewServeMux()
			ts := httptest.NewServer(mux)
			defer ts.Close()
			u, err := url.Parse(ts.URL)
			Expect(err).To(BeNil())
			port, err := strconv.Atoi(u.Port())
			Expect(err).To(BeNil())
			hostname := u.Hostname()

			config := DefaultConfig()
			Expect(config).ToNot(BeNil())
			config.SSL = false
			config.Hostname = hostname
			config.Port = port

			client := lucirpc{
				config:     config,
				httpClient: ts.Client(),
			}
			Expect(client).ToNot(BeNil())

			mux.HandleFunc(authPath, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			})

			err = client.auth(ctx)
			Expect(err).To(Equal(ErrHttpForbidden))
			Expect(client.token).To(Equal(""))
		})

		It("should fail", func() {
			mux := http.NewServeMux()
			ts := httptest.NewServer(mux)
			defer ts.Close()
			u, err := url.Parse(ts.URL)
			Expect(err).To(BeNil())
			port, err := strconv.Atoi(u.Port())
			Expect(err).To(BeNil())
			hostname := u.Hostname()

			config := DefaultConfig()
			Expect(config).ToNot(BeNil())
			config.SSL = false
			config.Hostname = hostname
			config.Port = port

			client := lucirpc{
				config:     config,
				httpClient: ts.Client(),
			}
			Expect(client).ToNot(BeNil())

			mux.HandleFunc(authPath, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			})

			err = client.auth(ctx)
			Expect(err).To(Equal(fmt.Errorf("http status code: 500")))
		})

		It("should fail with null token (wrong credentials)", func() {
			mux := http.NewServeMux()
			ts := httptest.NewServer(mux)
			defer ts.Close()
			u, err := url.Parse(ts.URL)
			Expect(err).To(BeNil())
			port, err := strconv.Atoi(u.Port())
			Expect(err).To(BeNil())

			config := DefaultConfig()
			config.SSL = false
			config.Hostname = u.Hostname()
			config.Port = port

			client := lucirpc{
				config:     config,
				httpClient: ts.Client(),
			}

			mux.HandleFunc(authPath, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"id":1,"result":null,"error":null}`))
			})

			err = client.auth(ctx)
			Expect(err).To(Equal(ErrRpcLoginFail))
			Expect(client.token).To(Equal(""))
		})

		It("should fail with empty token", func() {
			mux := http.NewServeMux()
			ts := httptest.NewServer(mux)
			defer ts.Close()
			u, err := url.Parse(ts.URL)
			Expect(err).To(BeNil())
			port, err := strconv.Atoi(u.Port())
			Expect(err).To(BeNil())

			config := DefaultConfig()
			config.SSL = false
			config.Hostname = u.Hostname()
			config.Port = port

			client := lucirpc{
				config:     config,
				httpClient: ts.Client(),
			}

			mux.HandleFunc(authPath, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"id":1,"result":"","error":null}`))
			})

			err = client.auth(ctx)
			Expect(err).To(Equal(ErrRpcLoginFail))
			Expect(client.token).To(Equal(""))
		})

	})

	Context("uci", func() {
		It("should re-auth and retry on 401", func() {
			mux := http.NewServeMux()
			ts := httptest.NewServer(mux)
			defer ts.Close()
			u, err := url.Parse(ts.URL)
			Expect(err).To(BeNil())
			port, err := strconv.Atoi(u.Port())
			Expect(err).To(BeNil())
			hostname := u.Hostname()

			config := DefaultConfig()
			Expect(config).ToNot(BeNil())
			config.Hostname = hostname
			config.Port = port
			config.SSL = false

			client := lucirpc{
				config:     config,
				httpClient: ts.Client(),
			}
			Expect(client).ToNot(BeNil())

			expectedToken := "foobar"
			authCalled := false
			mux.HandleFunc(authPath, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, err = w.Write([]byte(`{"result":"` + expectedToken + `"}`))
				Expect(err).To(BeNil())
			})

			expectedResp := "helloworld"
			mux.HandleFunc(uciPath, func(w http.ResponseWriter, r *http.Request) {
				// auth should be called
				if !authCalled {
					authCalled = true
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				Expect(r.URL.Path).To(Equal(uciPath))
				Expect(r.RequestURI).To(Equal(uciPath + "?auth=" + expectedToken))

				w.WriteHeader(http.StatusOK)
				_, err = w.Write([]byte(`{"result":"` + expectedResp + `"}`))
				Expect(err).To(BeNil())
			})

			resp, err := client.Uci(ctx, "get", []string{"network.lan.ipaddr"})
			Expect(err).To(BeNil())
			Expect(resp).To(Equal(expectedResp))
			Expect(authCalled).To(BeTrue())
			Expect(client.token).To(Equal(expectedToken))
		})

		It("should succeed without re-auth when already authenticated", func() {
			mux := http.NewServeMux()
			ts := httptest.NewServer(mux)
			defer ts.Close()
			u, err := url.Parse(ts.URL)
			Expect(err).To(BeNil())
			port, err := strconv.Atoi(u.Port())
			Expect(err).To(BeNil())

			config := DefaultConfig()
			config.Hostname = u.Hostname()
			config.Port = port
			config.SSL = false

			existingToken := "existing-token"
			client := lucirpc{
				config:     config,
				httpClient: ts.Client(),
				token:      existingToken,
			}

			expectedResp := "some-value"
			mux.HandleFunc(uciPath, func(w http.ResponseWriter, r *http.Request) {
				Expect(r.RequestURI).To(Equal(uciPath + "?auth=" + existingToken))
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"result":"` + expectedResp + `"}`))
			})

			resp, err := client.Uci(ctx, "get", []string{"dhcp"})
			Expect(err).To(BeNil())
			Expect(resp).To(Equal(expectedResp))
		})

		It("should return error from RPC error field", func() {
			mux := http.NewServeMux()
			ts := httptest.NewServer(mux)
			defer ts.Close()
			u, err := url.Parse(ts.URL)
			Expect(err).To(BeNil())
			port, err := strconv.Atoi(u.Port())
			Expect(err).To(BeNil())

			config := DefaultConfig()
			config.Hostname = u.Hostname()
			config.Port = port
			config.SSL = false

			client := lucirpc{
				config:     config,
				httpClient: ts.Client(),
				token:      "valid-token",
			}

			mux.HandleFunc(uciPath, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"id":1,"result":null,"error":"Invalid argument"}`))
			})

			resp, err := client.Uci(ctx, "get", []string{"nonexistent"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Invalid argument"))
			Expect(resp).To(Equal(""))
		})
	})
})
