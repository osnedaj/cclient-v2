package cclient_v2

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	tlsUtls "github.com/refraction-networking/utls"
	tlsHttp "github.com/useflyent/fhttp"
	tlsJar "github.com/useflyent/fhttp/cookiejar"
	"github.com/useflyent/fhttp/http2"
	"golang.org/x/net/proxy"
)

var (
	noTlsConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
)

func NewClient(proxyUrl string, timeout time.Duration, useTLS bool, optParams ...interface{}) (*Client, error) {
	jar, _ := tlsJar.New(nil)

	// handle non tls client
	if !useTLS {
		jar, _ := cookiejar.New(nil)

		transport := &http.Transport{
			ForceAttemptHTTP2: true,
			TLSClientConfig:   noTlsConfig,
		}

		if len(proxyUrl) > 0 {
			p, err := url.Parse(proxyUrl)
			if err != nil {
				return nil, err
			}
			transport.Proxy = http.ProxyURL(p)
		}

		return &Client{
			httpClient: &http.Client{
				Transport: transport,
				Jar:       jar,
				Timeout:   timeout,
			},
			http2Headers: nil,
			useTLS:       false,
			proxy:        proxyUrl,
		}, nil
	}

	if len(optParams) == 0 {
		log.Println("missing client hello when creating tls client")
		return nil, errors.New("missing client hello")
	}

	// handle tls client with proxy
	clientHello := optParams[0].(tlsUtls.ClientHelloID)
	var http2Headers map[http2.SettingID]uint32
	if len(optParams) == 1 {
		http2Headers = nil
	} else {
		http2Headers = optParams[1].(map[http2.SettingID]uint32)
	}

	if len(proxyUrl) > 0 {
		_, err := url.Parse(proxyUrl)
		if err != nil {
			return nil, err
		}

		dialer, err := newConnectDialer(proxyUrl)
		if err != nil {
			return nil, err
		}

		return &Client{
			tlsClient: &tlsHttp.Client{
				Jar:       jar,
				Transport: newRoundTripper(clientHello, http2Headers, dialer),
				Timeout:   timeout,
			},
			useTLS:       true,
			clientHello:  clientHello,
			http2Headers: http2Headers,
			proxy:        proxyUrl,
		}, nil
	}

	// handle tls client with no proxy
	return &Client{
		tlsClient: &tlsHttp.Client{
			Transport: newRoundTripper(clientHello, http2Headers, proxy.Direct),
			Timeout:   timeout,
			Jar:       jar,
		},
		useTLS:       true,
		http2Headers: http2Headers,
		proxy:        proxyUrl,
	}, nil

}

// SetMasterHeaderOrder sets header order for all requests, tls only
func (c *Client) SetMasterHeaderOrder(order []string) {
	if c.useTLS {
		c.MasterHeaderOrder = order
	}
}

func (c *Client) NewRequest() *Request {
	if c.useTLS {
		return &Request{
			Context: c.Context,
			useTLS:  true,
			TLSRequest: TLSRequest{
				client: c,
				header: make(tlsHttp.Header),
			},
		}
	}

	return &Request{
		Context: c.Context,
		useTLS:  false,
		HTTPRequest: HTTPRequest{
			client: c,
			header: make(http.Header),
		},
	}
}

// SetContext sets context for all requests
func (c *Client) SetContext(ctx context.Context) {
	c.Context = ctx
}

func (c *Client) GetCookies(cookieUrl string) []*http.Cookie {
	u, _ := url.Parse(cookieUrl)
	if c.useTLS {
		var cookies []*http.Cookie
		for _, i := range c.tlsClient.Jar.Cookies(u) {
			cookies = append(cookies, &http.Cookie{
				Name:       i.Name,
				Value:      i.Value,
				Path:       i.Path,
				Domain:     i.Domain,
				Expires:    i.Expires,
				RawExpires: i.RawExpires,
				MaxAge:     i.MaxAge,
				Secure:     i.Secure,
				HttpOnly:   i.HttpOnly,
				SameSite:   0,
				Raw:        i.Raw,
				Unparsed:   i.Unparsed,
			})
		}

		return cookies
	}

	return c.httpClient.Jar.Cookies(u)

}

func (c *Client) GetCookiesMap(cookieUrl string) map[string]string {
	u, _ := url.Parse(cookieUrl)

	cookieMap := make(map[string]string)
	if c.useTLS {
		for _, v := range c.tlsClient.Jar.Cookies(u) {
			cookieMap[v.Name] = v.Value
		}
		return cookieMap
	}

	for _, v := range c.httpClient.Jar.Cookies(u) {
		cookieMap[v.Name] = v.Value
	}
	return cookieMap

}

func (c *Client) GetCookieVal(cookieUrl string, cookieName string) (*http.Cookie, error) {
	u, _ := url.Parse(cookieUrl)

	if c.useTLS {
		for _, v := range c.tlsClient.Jar.Cookies(u) {
			if v.Name == cookieName {
				return &http.Cookie{
					Name:       v.Name,
					Value:      v.Value,
					Path:       v.Path,
					Domain:     v.Domain,
					Expires:    v.Expires,
					RawExpires: v.RawExpires,
					MaxAge:     v.MaxAge,
					Secure:     v.Secure,
					HttpOnly:   v.HttpOnly,
					SameSite:   0,
					Raw:        v.Raw,
					Unparsed:   v.Unparsed,
				}, nil
			}
		}
		return nil, errors.New("not found")
	}

	for _, v := range c.httpClient.Jar.Cookies(u) {
		if v.Name == cookieName {
			return v, nil
		}
	}
	return nil, errors.New("not found")
}

func (c *Client) UpdateProxy(p string) bool {
	if c.useTLS {
		if len(p) > 0 {
			dialer, err := newConnectDialer(p)
			if err != nil {
				return false
			}

			c.tlsClient.Transport = newRoundTripper(c.clientHello, c.http2Headers, dialer)
			c.proxy = p

			return true
		}

		c.tlsClient.Transport = newRoundTripper(c.clientHello, c.http2Headers, proxy.Direct)
		c.proxy = p

		return true
	}

	transport := &http.Transport{
		ForceAttemptHTTP2: true,
		TLSClientConfig:   noTlsConfig,
	}

	if len(p) > 0 {
		proxyUrl, _ := url.Parse(p)
		transport.Proxy = http.ProxyURL(proxyUrl)
	}

	c.proxy = p
	c.httpClient.Transport = transport

	return true
}

func (c *Client) SetCookieValue(cookieUrl string, cookieName string, value string, path ...string) {
	u, _ := url.Parse(cookieUrl)

	_path := "/"
	if len(path) > 0 {
		_path = path[0]
	}
	if c.useTLS {
		var cookies []*tlsHttp.Cookie
		newCookie := &tlsHttp.Cookie{
			Name:   cookieName,
			Value:  value,
			Domain: strings.Replace(u.Host, "www", "", -1),
			Path:   _path,
		}

		cookies = append(cookies, newCookie)
		c.tlsClient.Jar.SetCookies(u, cookies)
		return
	}

	var cookies []*http.Cookie
	newCookie := &http.Cookie{
		Name:   cookieName,
		Value:  value,
		Domain: strings.Replace(u.Host, "www", "", -1),
		Path:   _path,
	}
	cookies = append(cookies, newCookie)
	c.httpClient.Jar.SetCookies(u, cookies)
}

func (c *Client) SetCustomCookieValue(cookieData map[string]interface{}) {
	u, _ := url.Parse(cookieData["url"].(string))

	domain := strings.Replace(u.Host, "www", "", -1)
	if newDomain, ok := cookieData["domain"]; ok {
		domain = newDomain.(string)
	}

	if c.useTLS {
		var cookies []*tlsHttp.Cookie
		newCookie := &tlsHttp.Cookie{
			Name:   cookieData["name"].(string),
			Value:  cookieData["value"].(string),
			Domain: domain,
			Path:   "/",
		}

		if cookieData["expires"] != nil {
			newCookie.Expires = cookieData["expires"].(time.Time).UTC()
		}
		if cookieData["path"] != nil {
			newCookie.Path = cookieData["path"].(string)
		}
		if cookieData["maxAge"] != nil {
			newCookie.MaxAge = cookieData["maxAge"].(int)
		}

		cookies = append(cookies, newCookie)
		c.tlsClient.Jar.SetCookies(u, cookies)
		return
	}

	var cookies []*http.Cookie
	newCookie := &http.Cookie{
		Name:   cookieData["name"].(string),
		Value:  cookieData["value"].(string),
		Domain: domain,
		Path:   "/",
	}

	if cookieData["expires"] != nil {
		newCookie.Expires = cookieData["expires"].(time.Time).UTC()
	}
	if cookieData["path"] != nil {
		newCookie.Path = cookieData["path"].(string)
	}
	if cookieData["maxAge"] != nil {
		newCookie.MaxAge = cookieData["maxAge"].(int)
	}

	cookies = append(cookies, newCookie)
	c.httpClient.Jar.SetCookies(u, cookies)
}

func (c *Client) ResetCookies() {
	if c.useTLS {
		jar, _ := tlsJar.New(nil)
		c.tlsClient.Jar = jar
		return
	}

	jar, _ := cookiejar.New(nil)
	c.httpClient.Jar = jar
}

func (c *Client) RemoveCookie(siteUrl string, cookieName string) {
	u, _ := url.Parse(siteUrl)
	if c.useTLS {
		cookies := c.tlsClient.Jar.Cookies(u)
		for _, cookie := range cookies {
			if strings.ToLower(cookie.Name) == strings.ToLower(cookieName) && cookie.MaxAge != -1 {
				cookie.MaxAge = -1
				cookie.Expires = time.Now().Add(time.Hour * -100)
			}
		}
		c.tlsClient.Jar.SetCookies(u, cookies)
		return
	}

	cookies := c.httpClient.Jar.Cookies(u)
	for _, cookie := range cookies {
		if strings.ToLower(cookie.Name) == strings.ToLower(cookieName) && cookie.MaxAge != -1 {
			cookie.MaxAge = -1
			cookie.Expires = time.Now().Add(time.Hour * -100)
		}
	}
	c.httpClient.Jar.SetCookies(u, cookies)
}

func (c *Client) SetHeaderSettings() {

}

// Do will send the specified request
func (c *Client) Do(tlsRequest *tlsHttp.Request, httpRequest *http.Request, useTLS bool) (*Response, error) {
	if useTLS {
		resp, err := c.tlsClient.Do(tlsRequest)
		if err != nil {
			return nil, err
		}

		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var headers = Header{}
		for k, v := range resp.Header {
			headers[k] = v
		}

		var cookies []*http.Cookie
		for _, v := range resp.Cookies() {
			cookies = append(cookies, &http.Cookie{
				Name:       v.Name,
				Value:      v.Value,
				Path:       v.Path,
				Domain:     v.Domain,
				Expires:    v.Expires,
				RawExpires: v.RawExpires,
				MaxAge:     v.MaxAge,
				Secure:     v.Secure,
				HttpOnly:   v.HttpOnly,
				SameSite:   http.SameSite(v.SameSite),
				Raw:        v.Raw,
				Unparsed:   v.Unparsed,
			})
		}

		var requestCookies []*http.Cookie
		for _, v := range tlsRequest.Cookies() {
			cookies = append(cookies, &http.Cookie{
				Name:       v.Name,
				Value:      v.Value,
				Path:       v.Path,
				Domain:     v.Domain,
				Expires:    v.Expires,
				RawExpires: v.RawExpires,
				MaxAge:     v.MaxAge,
				Secure:     v.Secure,
				HttpOnly:   v.HttpOnly,
				SameSite:   http.SameSite(v.SameSite),
				Raw:        v.Raw,
				Unparsed:   v.Unparsed,
			})
		}

		response := &Response{
			request:        transformRequest(tlsRequest),
			requestCookies: requestCookies,
			cookies:        cookies,
			headers:        headers,
			body:           body,
			status:         resp.Status,
			reqUrl:         resp.Request.URL,
			statusCode:     resp.StatusCode,
		}
		return response, nil
	}

	resp, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var headers = Header{}
	for k, v := range resp.Header {
		headers[k] = v
	}

	response := &Response{
		requestCookies: httpRequest.Cookies(),
		request:        httpRequest,
		headers:        headers,
		cookies:        resp.Cookies(),
		body:           body,
		status:         resp.Status,
		reqUrl:         resp.Request.URL,
		statusCode:     resp.StatusCode,
	}

	return response, nil
}

func transformResponse(r *tlsHttp.Response) *http.Response {
	var headers = http.Header{}
	for k, v := range r.Header {
		headers[k] = v
	}

	return &http.Response{
		Status:           r.Status,
		StatusCode:       r.StatusCode,
		Proto:            r.Proto,
		ProtoMajor:       r.ProtoMajor,
		ProtoMinor:       r.ProtoMinor,
		Header:           headers,
		Body:             r.Body,
		ContentLength:    r.ContentLength,
		TransferEncoding: r.TransferEncoding,
		Close:            r.Close,
		Uncompressed:     r.Uncompressed,
		Request:          transformRequest(r.Request),
	}
}

func transformRequest(r *tlsHttp.Request) *http.Request {
	var headers = http.Header{}
	for k, v := range r.Header {
		headers[k] = v
	}

	return &http.Request{
		Method:           r.Method,
		URL:              r.URL,
		Proto:            r.Proto,
		ProtoMajor:       r.ProtoMajor,
		ProtoMinor:       r.ProtoMinor,
		Header:           headers,
		Body:             r.Body,
		GetBody:          r.GetBody,
		ContentLength:    r.ContentLength,
		TransferEncoding: r.TransferEncoding,
		Close:            r.Close,
		Host:             r.Host,
		Form:             r.Form,
		PostForm:         r.PostForm,
		MultipartForm:    r.MultipartForm,
		RemoteAddr:       r.RemoteAddr,
		RequestURI:       r.RequestURI,
	}
}
