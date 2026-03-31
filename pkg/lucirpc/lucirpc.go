package lucirpc

//go:generate mockgen -destination=../../internal/mocks/lucirpc/lucirpc.go -package=mocks . LuciRPC

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/yukariin/external-dns-openwrt-webhook/pkg/logger"
	"go.uber.org/zap"
)

const (
	rpcPath  = "/cgi-bin/luci/rpc/"
	authPath = rpcPath + "auth"
	uciPath  = rpcPath + "uci"

	methodLogin = "login"
)

var (
	ErrRpcLoginFail = errors.New("rpc: login fail")

	ErrHttpUnauthorized = errors.New("http: Unauthorized")
	ErrHttpForbidden    = errors.New("http: Forbidden")
)

type LuciRPC interface {
	Uci(context.Context, string, []string) (string, error)
}

type Payload struct {
	ID     int      `json:"id"`
	Method string   `json:"method"`
	Params []string `json:"params"`
}

type Response struct {
	ID     int         `json:"id"`
	Result interface{} `json:"result"`
	Error  interface{} `json:"error"`
}

type lucirpc struct {
	config     *Config
	token      string
	httpClient *http.Client
}

func New(config *Config) (LuciRPC, error) {
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: config.InsecureSkipVerify,
			},
			DialContext: (&net.Dialer{
				Timeout:   time.Duration(config.Timeout) * time.Second,
				KeepAlive: time.Duration(config.Timeout) * time.Second,
			}).DialContext,
		},
	}

	return &lucirpc{
		config:     config,
		httpClient: httpClient,
	}, nil
}

func (c *lucirpc) Uci(ctx context.Context, method string, params []string) (string, error) {
	return c.rpcWithAuth(ctx, uciPath, method, params)
}

func (c *lucirpc) auth(ctx context.Context) error {
	token, err := c.rpc(ctx, authPath, methodLogin, []string{c.config.Auth.Username, c.config.Auth.Password})
	if err != nil {
		logger.Log.Error("rpc: login fail", zap.Error(err))
		return err
	}

	// OpenWRT JSON RPC response of wrong username and password
	// {"id":1,"result":null,"error":null}
	if token == "" || token == "null" {
		return ErrRpcLoginFail
	}

	c.token = token
	return nil
}

func (c *lucirpc) rpc(ctx context.Context, path, method string, params []string) (string, error) {
	data, err := json.Marshal(Payload{
		ID:     c.config.RpcID,
		Method: method,
		Params: params,
	})
	if err != nil {
		logger.Log.Error("marshal fail", zap.Error(err))
		return "", err
	}

	url := c.getUri(path, method)
	respBody, err := c.call(ctx, url, data)
	if err != nil {
		logger.Log.Error("call fail", zap.Error(err))
		return "", err
	}

	var response Response
	if err := json.Unmarshal(respBody, &response); err != nil {
		logger.Log.Error("unmarshal fail", zap.Error(err))
		return "", err
	}

	if response.Error != nil {
		return "", parseError(response.Error)
	}

	if response.Result != nil {
		return parseString(response.Result)
	}

	return "", nil
}

func (c *lucirpc) getUri(path, method string) string {
	logger.Log.Debug("uri", zap.String("path", path), zap.String("method", method))
	proto := "https://"
	if !c.config.SSL {
		proto = "http://"
	}

	url := proto + c.config.Hostname + ":" + strconv.Itoa(c.config.Port) + path
	if method != methodLogin && c.token != "" {
		url = url + "?auth=" + c.token
	}

	return url
}

func (c *lucirpc) call(ctx context.Context, rawURL string, postBody []byte) ([]byte, error) {
	if u, err := url.Parse(rawURL); err == nil {
		logger.Log.Debug("call", zap.String("path", u.Path))
	}
	body := bytes.NewReader(postBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode > 226 {
		return respBody, c.httpError(resp.StatusCode)
	}

	return respBody, nil
}

func (c *lucirpc) httpError(code int) error {
	if code == 401 {
		return ErrHttpUnauthorized
	}

	if code == 403 {
		return ErrHttpForbidden
	}

	return fmt.Errorf("http status code: %d", code)
}

func (c *lucirpc) rpcWithAuth(ctx context.Context, path, method string, params []string) (string, error) {
	result, err := c.rpc(ctx, path, method, params)
	if err == nil {
		return result, nil
	}

	if err != ErrHttpUnauthorized && err != ErrHttpForbidden {
		return "", err
	}

	logger.Log.Info("re-authenticate")
	if err = c.auth(ctx); err != nil {
		return "", err
	}

	return c.rpc(ctx, path, method, params)
}

func parseString(obj interface{}) (string, error) {
	if obj == nil {
		return "", errors.New("nil object cannot be parsed")
	}

	if s, ok := obj.(string); ok {
		return s, nil
	}

	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

func parseError(obj interface{}) error {
	result, err := parseString(obj)
	if err != nil {
		return err
	}

	return errors.New(result)
}
