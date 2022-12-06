package cclient_v2

import (
	"context"
	"io"
	"net/http"
	"net/textproto"
	"net/url"

	tlsUtls "github.com/refraction-networking/utls"
	tlsHttp "github.com/useflyent/fhttp"
	"github.com/useflyent/fhttp/http2"
)

type Client struct {
	Context           context.Context
	proxy             string
	useTLS            bool
	MasterHeaderOrder []string
	http2Headers      map[http2.SettingID]uint32
	tlsClient         *tlsHttp.Client
	clientHello       tlsUtls.ClientHelloID
	httpClient        *http.Client
}

// Request base request struct
type Request struct {
	useTLS      bool
	Context     context.Context
	HeaderOrder []string
	TLSRequest  TLSRequest
	HTTPRequest HTTPRequest
}

// TLSRequest tls request struct
type TLSRequest struct {
	client            *Client
	method, url, host string
	header            tlsHttp.Header
	body              io.Reader
	cookies           []*tlsHttp.Cookie
}

// HTTPRequest non-tls request struct
type HTTPRequest struct {
	client            *Client
	method, url, host string
	header            http.Header
	body              io.Reader
	cookies           []*http.Cookie
}

// Response base response struct
type Response struct {
	request        *http.Request
	requestCookies []*http.Cookie
	cookies        []*http.Cookie
	IsTLS          bool
	headers        Header
	body           []byte
	reqUrl         *url.URL
	status         string
	statusCode     int
}

type Header map[string][]string

func (h Header) Get(key string) string {
	return textproto.MIMEHeader(h).Get(key)
}

func (h Header) Values(key string) []string {
	return textproto.MIMEHeader(h).Values(key)
}
