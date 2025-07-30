package watchbook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type URL = url.URL

type ClientUrls struct {
	addr string
}

func (c *ClientUrls) getUrl(path string) (*url.URL, error) {
	return createUrlBase(c.addr, path, nil)
}

type Client struct {
	Url     ClientUrls
	Headers http.Header
	addr    string
}

func New(addr string) *Client {
	return &Client{
		Url: ClientUrls{
			addr: addr,
		},
		Headers: map[string][]string{},
		addr:    addr,
	}
}

type Options struct {
	Query  url.Values
	Header http.Header
}

func createUrlBase(addr, path string, query url.Values) (*url.URL, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	u.Path = path

	if query != nil {
		params := u.Query()
		for k, v := range query {
			params[k] = v
		}
		u.RawQuery = params.Encode()
	}

	return u, nil
}

func createUrl(addr, path string, query url.Values) (string, error) {
	url, err := createUrlBase(addr, path, query)
	if err != nil {
		return "", err
	}

	return url.String(), nil
}

type ApiError[E any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type"`
	Extra   E      `json:"extra,omitempty"`
}

func (err *ApiError[E]) Error() string {
	return err.Message
}

type ApiResponse[D any, E any] struct {
	Success bool         `json:"success"`
	Data    D            `json:"data,omitempty"`
	Error   *ApiError[E] `json:"error,omitempty"`
}

type RequestData struct {
	Url    string
	Method string

	ClientHeaders http.Header
	Headers       http.Header
}

func rawRequest(
	data *RequestData,
	contentType string,
	bodyReader io.Reader,
) (*http.Response, error) {
	req, err := http.NewRequest(data.Method, data.Url, bodyReader)
	if err != nil {
		return nil, err
	}

	newHeaders := data.ClientHeaders.Clone()
	newHeaders.Set("Content-Type", contentType)

	for k, v := range data.Headers {
		newHeaders[k] = v
	}

	req.Header = newHeaders

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func Request[D any](data RequestData, body any) (*D, error) {
	var bodyReader io.Reader

	if body != nil {
		buf := bytes.Buffer{}

		err := json.NewEncoder(&buf).Encode(body)
		if err != nil {
			return nil, err
		}

		bodyReader = &buf
	}

	resp, err := rawRequest(&data, "application/json", bodyReader)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res ApiResponse[D, any]
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, err
	}

	if !res.Success {
		return nil, res.Error
	}

	return &res.Data, nil
}

// NOTE(patrik): Copied from multipart.Writer.FormDataContentType
func createFormContentType(b string) string {
	if strings.ContainsAny(b, `()<>@,;:\"/[]?= `) {
		b = `"` + b + `"`
	}
	return "multipart/form-data; boundary=" + b
}

func RequestForm[D any](data RequestData, boundary string, body Reader) (*D, error) {
	ct := createFormContentType(boundary)
	resp, err := rawRequest(&data, ct, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res ApiResponse[D, any]
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, err
	}

	if !res.Success {
		return nil, res.Error
	}

	return &res.Data, nil
}

// Simple wrapper for Sprintf
func Sprintf(format string, a ...any) string {
	return fmt.Sprintf(format, a...)
}

// Copy of io.Reader interface
type Reader interface {
	Read(p []byte) (n int, err error)
}
