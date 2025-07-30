package plugins

import (
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

type ClientHTTP string

var (
	fxs map[string]interface{} = make(map[string]interface{})
)

func (d ClientHTTP) Run(c echo.Context,
	vars map[string]string, payloadIn interface{}, dromedaryData string,
	callback chan string,
) (payloadOut interface{}, next string, err error) {
	return nil, "output_1", nil
}

func init() {
	addFeatureHttp()
}

func (d ClientHTTP) AddFeatureJS() map[string]interface{} {
	return fxs
}

func (d ClientHTTP) Name() string {
	return "client_http"
}

func httpRequest(method string, url string, body *string, header map[string][]string) map[string]interface{} {

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{
		Transport: tr,
	}

	// Create an http.Request instance
	var req *http.Request
	if method == http.MethodGet {
		req, _ = http.NewRequest(method, url, nil)
	} else if method == http.MethodDelete {
		req, _ = http.NewRequest(method, url, nil)
	} else {
		rb := strings.NewReader(*body)
		req, _ = http.NewRequest(method, url, rb)
	}

	if len(header) > 0 {
		req.Header = header
	}

	res, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	rbody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return map[string]interface{}{"body": "", "err": err, "status": res.Status, "header": res.Header, "status_code": res.StatusCode}
	}

	return map[string]interface{}{"body": string(rbody), "err": err, "status": res.Status, "header": res.Header, "status_code": res.StatusCode}
}

func httpGet(url string) map[string]interface{} {
	return httpRequest(http.MethodGet, url, nil, map[string][]string{})
}

func httpDelete(url string) map[string]interface{} {
	return httpRequest(http.MethodDelete, url, nil, map[string][]string{})
}

func httpPost(url string, body string) map[string]interface{} {
	return httpRequest(http.MethodPost, url, &body, map[string][]string{})
}
func httpPut(url string, body string) map[string]interface{} {
	return httpRequest(http.MethodPut, url, &body, map[string][]string{})
}
func httpPatch(url string, body string) map[string]interface{} {
	return httpRequest(http.MethodPatch, url, &body, map[string][]string{})
}

func httpGetWithHeader(url string, header map[string][]string) map[string]interface{} {
	return httpRequest(http.MethodGet, url, nil, header)
}

func httpDeleteWithHeader(url string, header map[string][]string) map[string]interface{} {
	return httpRequest(http.MethodDelete, url, nil, header)
}

func httpPostWithHeader(url string, body string, header map[string][]string) map[string]interface{} {
	return httpRequest(http.MethodPost, url, &body, header)
}
func httpPutWithHeader(url string, body string, header map[string][]string) map[string]interface{} {
	return httpRequest(http.MethodPut, url, &body, header)
}
func httpPatchWithHeader(url string, body string, header map[string][]string) map[string]interface{} {
	return httpRequest(http.MethodPatch, url, &body, header)
}

func addFeatureHttp() {
	fxs["http_get"] = httpGet
	fxs["http_post"] = httpPost
	fxs["http_delete"] = httpDelete
	fxs["http_put"] = httpPut
	fxs["http_patch"] = httpPatch
	fxs["http_get_with_header"] = httpGetWithHeader
	fxs["http_post_with_header"] = httpPostWithHeader
	fxs["http_delete_with_header"] = httpDeleteWithHeader
	fxs["http_put_with_header"] = httpPutWithHeader
	fxs["http_patch_with_header"] = httpPatchWithHeader
}
