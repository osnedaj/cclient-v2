package cclient_v2

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"github.com/gabriel-vasile/mimetype"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/andybalholm/brotli"
)

// Header returns the response headers
func (r *Response) Header() Header {
	return r.headers
}

// Cookies returns the response cookies
func (r *Response) Cookies() []*http.Cookie {
	return r.cookies
}

// Body returns the response body
func (r *Response) Body(params ...string) []byte {
	encoding := r.headers.Get("Content-Encoding")
	body := r.body

	mtype := mimetype.Detect(body)
	println(mtype.String())

	if len(encoding) > 0 || len(encoding) == 0 && len(params) > 0 {
		if len(params) > 0 {
			encoding = params[0]
		}
		if encoding == "gzip" {
			unz, err := gUnzipData(body)
			if err != nil {
				fmt.Println(err)
			}
			return unz
		} else if encoding == "deflate" {
			unz, err := enflateData(body)
			if err != nil {
				fmt.Println(err)
			}
			return unz
		} else if encoding == "br" {
			unz, err := unBrotliData(body)
			if err != nil {
				fmt.Println(err)
			}
			return unz
		} else {
			fmt.Println("UNKNOWN ENCODING: " + encoding)
		}
	}
	return body
}

// ReqUrl returns the response URL
func (r *Response) ReqUrl() *url.URL {
	return r.reqUrl
}

func (r *Response) RequestCookies() []*http.Cookie {
	return r.requestCookies
}

// BodyAsString returns the response body as a string
func (r *Response) BodyAsString(params ...string) string {
	body := r.Body(params...)
	return string(body)
}

func gUnzipData(data []byte) (resData []byte, err error) {
	gz, _ := gzip.NewReader(bytes.NewReader(data))
	defer gz.Close()
	respBody, err := ioutil.ReadAll(gz)
	return respBody, err
}
func enflateData(data []byte) (resData []byte, err error) {
	zr, _ := zlib.NewReader(bytes.NewReader(data))
	defer zr.Close()
	enflated, err := ioutil.ReadAll(zr)
	return enflated, err
}

func unBrotliData(data []byte) (resData []byte, err error) {
	br := brotli.NewReader(bytes.NewReader(data))
	respBody, err := ioutil.ReadAll(br)
	return respBody, err
}

// BodyAsJSON unmarshalls the current response body to the specified data structure
func (r *Response) BodyAsJSON(data interface{}) error {
	encoding := r.headers.Get("Content-Encoding")
	mainBody := r.body
	if len(encoding) > 0 {
		if encoding == "gzip" {
			body, err := gUnzipData(mainBody)
			if err != nil {
				fmt.Println(err)
			}
			mainBody = body
		} else if encoding == "deflate" {
			body, err := enflateData(mainBody)
			if err != nil {
				fmt.Println(err)
			}
			mainBody = body
		} else if encoding == "br" {
			body, err := unBrotliData(mainBody)
			if err != nil {
				fmt.Println(err)
			}
			mainBody = body
		}
	}
	return json.Unmarshal(mainBody, data)
}

// Request returns the request
func (r *Response) Request() *http.Request {
	return r.request
}

// Status returns the response status
func (r *Response) Status() string {
	return r.status
}

// StatusCode returns the response status code
func (r *Response) StatusCode() int {
	return r.statusCode
}
