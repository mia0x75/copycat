package http

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

type Http struct {
	url string
}

const (
	HTTP_POST_TIMEOUT = 6 // default timeout for data posting
)

var tr = &http.Transport{
	MaxIdleConnsPerHost: 64,
	TLSClientConfig: &tls.Config{
		InsecureSkipVerify: true,
	},
	DisableCompression:  false,
	DisableKeepAlives:   false,
	TLSHandshakeTimeout: 10 * time.Second,
	Dial: func(netw, addr string) (net.Conn, error) {
		dial := net.Dialer{
			Timeout:   HTTP_POST_TIMEOUT * time.Second,
			KeepAlive: 600 * time.Second,
		}
		conn, err := dial.Dial(netw, addr)
		if err != nil {
			return conn, err
		}
		return conn, nil
	},
}

var defaultHttpClient = http.Client{
	Transport: tr,
}

func NewHttp(url string) *Http {
	request := &Http{
		url: url,
	}
	return request
}

func (req *Http) Put(data []byte) ([]byte, error) {
	return request("PUT", req.url, data)
}

func (req *Http) Post(data []byte) ([]byte, error) {
	return request("POST", req.url, data)
}

func (req *Http) Get() ([]byte, error) {
	return request("GET", req.url, nil)
}

func (req *Http) Delete() ([]byte, error) {
	return request("DELETE", req.url, nil)
}

func Post(url string, data []byte) ([]byte, error) {
	c := NewHttp(url)
	return c.Post(data)
}

func Get(url string) ([]byte, error) {
	c := NewHttp(url)
	return c.Get()
}

func request(method string, url string, data []byte) ([]byte, error) {
	reader := bytes.NewReader(data)
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connection", "keep-alive")
	resp, err := defaultHttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("HTTP return code: %d.", resp.StatusCode)) //ErrorHttpStatus
	}
	res, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	io.Copy(ioutil.Discard, resp.Body)

	return res, nil
}
