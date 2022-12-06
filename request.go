package cclient_v2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	tlsHttp "github.com/useflyent/fhttp"
)

// SetURL sets the url of the request
func (r *Request) SetURL(url string) *Request {
	if r.useTLS {
		r.TLSRequest.url = url
	} else {
		r.HTTPRequest.url = url
	}

	return r
}

// SetMethod sets the method of the request
func (r *Request) SetMethod(method string) *Request {
	if r.useTLS {
		r.TLSRequest.method = method
	} else {
		r.HTTPRequest.method = method
	}

	return r
}

// AddHeader adds a specified header to the request
// If the header already exists, the value will be appended by the new specified value
// If the header does not exist, the header will be set to the specified value
func (r *Request) AddHeader(key, value string) *Request {
	if len(value) == 0 {
		return r
	}

	if r.useTLS {
		if header, ok := r.TLSRequest.header[key]; ok {
			header = append(header, value)
			r.TLSRequest.header[key] = header
		} else {
			r.TLSRequest.header[key] = []string{value}
		}
	} else {
		if header, ok := r.HTTPRequest.header[key]; ok {
			header = append(header, value)
			r.HTTPRequest.header[key] = header
		} else {
			r.HTTPRequest.header[key] = []string{value}
		}
	}

	return r
}

func (r *Request) SetSecHeaders(version ...string) *Request {
	var v string
	if len(version) == 0 {
		v = "99"
	} else {
		v = version[0]
	}

	return r.AddHeader("sec-ch-ua", fmt.Sprintf("\" Not A;Brand\";v=\"%s\", \"Chromium\";v=\"%s\", \"Google Chrome\";v=\"%s\"", v, v, v)).
		AddHeader("sec-ch-ua-mobile", "?0").
		AddHeader("sec-ch-ua-platform", "\"Windows\"").
		AddHeader("sec-fetch-dest", "").
		AddHeader("sec-fetch-dest", "document").
		AddHeader("sec-fetch-mode", "navigate").
		AddHeader("sec-fetch-site", "none")
}

func (r *Request) SetUserAgent(ua string) *Request {
	return r.AddHeader("user-agent", ua)
}

func (r *Request) SetHeaders(headers map[string]string) *Request {
	for k, v := range headers {
		r.SetHeader(k, v)
	}

	return r
}

// SetHeader sets a specified header to the request
// This overrides any previously set values of the specified header
func (r *Request) SetHeader(key, value string) *Request {
	if len(value) == 0 {
		return r
	}

	if strings.Contains(key, "x-instana") {
		return r
	}

	if r.useTLS {
		r.TLSRequest.header[key] = []string{value}
	} else {
		r.HTTPRequest.header[key] = []string{value}
	}
	return r
}

// SetHost sets the host of the request
func (r *Request) SetHost(value string) *Request {
	if r.useTLS {
		r.TLSRequest.host = value
	} else {
		r.HTTPRequest.host = value
	}

	return r
}

// SetJSONBody sets the body to a json value
func (r *Request) SetJSONBody(body interface{}) *Request {
	b, _ := json.Marshal(body)
	if r.useTLS {
		r.TLSRequest.body = bytes.NewBuffer(b)
	} else {
		r.HTTPRequest.body = bytes.NewBuffer(b)
	}

	return r
}

func (r *Request) SetBody(body string) *Request {
	if r.useTLS {
		r.TLSRequest.body = strings.NewReader(body)
	} else {
		r.HTTPRequest.body = strings.NewReader(body)
	}

	return r
}

// SetFormBody sets the body to a form value
func (r *Request) SetFormBody(body url.Values) *Request {
	b := strings.NewReader(body.Encode())
	if r.useTLS {
		r.TLSRequest.body = b
	} else {
		r.HTTPRequest.body = b
	}

	return r
}

// SetHeaderOrder sets the http header order, only works for tls requests
func (r *Request) SetHeaderOrder(order []string) *Request {
	if r.useTLS {
		//r.HeaderOrder = order
	}

	return r
}

// Do will send the request with all specified request values
func (r *Request) Do() (*Response, error) {
	if r.useTLS {
		req, err := tlsHttp.NewRequest(r.TLSRequest.method, r.TLSRequest.url, r.TLSRequest.body)

		if err != nil {
			return nil, err
		}

		var headerOrder []string

		if len(r.HeaderOrder) != 0 {
			// request specific header order - override master header order
			headerOrder = r.HeaderOrder
		} else {
			// default header order
			headerOrder = []string{
				"host",
				"connection",
				"cache-control",
				"device-memory",
				"viewport-width",
				"rtt",
				"downlink",
				"ect",
				"sec-ch-ua",
				"sec-ch-ua-mobile",
				"sec-ch-ua-full-version",
				"sec-ch-ua-arch",
				"sec-ch-ua-platform",
				"sec-ch-ua-platform-version",
				"sec-ch-ua-model",
				"upgrade-insecure-requests",
				"user-agent",
				"accept",
				"sec-fetch-site",
				"sec-fetch-mode",
				"sec-fetch-user",
				"sec-fetch-dest",
				"referer",
				"accept-encoding",
				"accept-language",
				"cookie",
				"content-type",
				"authorization",
			}

			// override default header order with master header order
			if len(r.TLSRequest.client.MasterHeaderOrder) != 0 {
				headerOrder = r.TLSRequest.client.MasterHeaderOrder
			}
		}

		headerMap := make(map[string]string)

		var headerOrderKey []string
		for _, key := range headerOrder {
			headerOrderKey = append(headerOrderKey, key)
			for k, v := range r.TLSRequest.header {
				lowerCaseKey := strings.ToLower(k)
				if key == lowerCaseKey {
					headerMap[k] = v[0]
				}
			}
		}

		for k, v := range req.Header {
			if _, ok := headerMap[k]; !ok {
				headerMap[k] = v[0]
				headerOrderKey = append(headerOrderKey, strings.ToLower(k))
			}
		}

		req.Header = tlsHttp.Header{
			tlsHttp.HeaderOrderKey:  headerOrderKey,
			tlsHttp.PHeaderOrderKey: {":method", ":authority", ":scheme", ":path"},
		}

		//u, err := url.Parse(r.TLSRequest.url)
		//if err != nil {
		//	panic(err)
		//}

		for k, v := range r.TLSRequest.header {
			req.Header.Set(k, v[0])
		}

		if len(r.TLSRequest.host) > 0 {
			req.Host = r.TLSRequest.host
		} else {
			//req.Header.Set("Host", u.Host)
		}

		if r.Context != nil {
			req = req.WithContext(r.Context)
		}

		return r.TLSRequest.client.Do(req, nil, true)
	}

	req, err := http.NewRequest(r.HTTPRequest.method, r.HTTPRequest.url, r.HTTPRequest.body)

	if err != nil {
		return nil, err
	}

	for _, cookie := range r.HTTPRequest.cookies {
		if cookie != nil {
			req.AddCookie(cookie)
		}
	}

	req.Header = r.HTTPRequest.header

	if len(r.HTTPRequest.host) > 0 {
		req.Host = r.HTTPRequest.host
	}

	if r.Context != nil {
		req = req.WithContext(r.Context)
	}

	return r.HTTPRequest.client.Do(nil, req, false)
}
