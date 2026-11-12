package native

import (
	"fmt"
	"go_js/heap"
	"go_js/object"
	"go_js/value"
	"io"
	"net/http"
	"strings"
	"time"
)

type ObjFetchHeaders struct {
	ObjObject
	Header http.Header
}

type ObjFetchResponse struct {
	ObjObject
	Status     int
	StatusText string
	Body       []byte
	Header     http.Header
	URL        string
}

func NewFetchHeaders(header http.Header) *ObjFetchHeaders {
	headers := &ObjFetchHeaders{Header: header}
	headers.Members = map[string]ObjectValueEntry{}
	headers.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	headers.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("get"))), hostFunction("get", HOST_HEADERS_GET))
	headers.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("has"))), hostFunction("has", HOST_HEADERS_HAS))
	headers.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("entries"))), hostFunction("entries", HOST_HEADERS_ENTRIES))
	return headers
}

func (*ObjFetchHeaders) Type() object.ObjType { return object.OBJ_OBJECT }
func (*ObjFetchHeaders) String() string       { return "[object Headers]" }

func NewFetchResponse(status int, statusText string, body []byte, header http.Header, url string) *ObjFetchResponse {
	response := &ObjFetchResponse{Status: status, StatusText: statusText, Body: body, Header: header, URL: url}
	response.Members = map[string]ObjectValueEntry{}
	response.SetMember(KEY_PROTO, PROTOTYPE_OBJECT)
	response.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("status"))), value.ValueFromFloat64(float64(status)))
	response.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("statusText"))), value.EncodeHandle(heap.Allocate(NewLightString(statusText))))
	response.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("ok"))), boolValueValue(status >= 200 && status < 300))
	response.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("url"))), value.EncodeHandle(heap.Allocate(NewLightString(url))))
	response.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("redirected"))), value.FALSE)
	response.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("type"))), value.EncodeHandle(heap.Allocate(NewLightString("basic"))))
	response.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("bodyUsed"))), value.FALSE)
	response.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("headers"))), value.EncodeHandle(heap.Allocate(NewFetchHeaders(header))))
	response.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("text"))), hostFunction("text", HOST_RESPONSE_TEXT))
	response.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("json"))), hostFunction("json", HOST_RESPONSE_JSON))
	response.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("arrayBuffer"))), hostFunction("arrayBuffer", HOST_RESPONSE_ARRAY_BUFFER))
	response.SetMember(value.EncodeHandle(heap.Allocate(NewLightString("clone"))), hostFunction("clone", HOST_RESPONSE_CLONE))
	return response
}

func (*ObjFetchResponse) Type() object.ObjType { return object.OBJ_OBJECT }
func (*ObjFetchResponse) String() string       { return "[object Response]" }

func FetchHTTP(rawURL, method string, header http.Header, body []byte) (*ObjFetchResponse, error) {
	if method == "" {
		method = http.MethodGet
	}
	request, err := http.NewRequest(strings.ToUpper(method), rawURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	if header == nil {
		header = http.Header{}
	}
	request.Header = header.Clone()
	if request.Header.Get("User-Agent") == "" {
		request.Header.Set("User-Agent", "go_js")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	return NewFetchResponse(response.StatusCode, response.Status, data, response.Header.Clone(), response.Request.URL.String()), nil
}

var _ = fmt.Sprintf
