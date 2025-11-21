package sip

import (
	"bytes"
	"fmt"

	"github.com/gofrs/uuid"
)

// Response Response
type Response struct {
	message
	statusCode int
	reason     string
}

// NewResponseFromRequest NewResponseFromRequest
func NewResponseFromRequest(
	resID MessageID,
	req *Request,
	statusCode int,
	reason string,
	body []byte,
) *Response {
	res := NewResponse(
		resID,
		req.SipVersion(),
		statusCode,
		reason,
		[]Header{},
		[]byte{},
	)

	CopyHeaders("Record-Route", req, res)
	CopyHeaders("Via", req, res)
	CopyHeaders("From", req, res)
	to, ok := req.To()
	if ok {
		if _, ok := to.Params.Get("tag"); !ok {
			to.Params.Add("tag", String{Str: RandString(32)})
		}
	}
	CopyHeaders("CSeq", req, res)
	CopyHeaders("Call-ID", req, res)

	res.AppendHeader(to)

	if statusCode == 100 {
		CopyHeaders("Timestamp", req, res)
	}

	res.SetSource(req.Destination())
	res.SetDestination(req.Source())

	res.SetBody(body, true)

	res.AppendHeader(&GenericHeader{
		HeaderName: "User-Agent",
		Contents:   "GoWVP/1.0",
	})

	return res
}

// NewResponse NewResponse
func NewResponse(
	messID MessageID,
	sipVersion string,
	statusCode int,
	reason string,
	hdrs []Header,
	body []byte,
) *Response {
	res := new(Response)
	if messID == "" {
		res.messID = MessageID(uuid.Must(uuid.NewV4()).String())
	} else {
		res.messID = messID
	}
	res.startLine = res.StartLine
	res.SetSipVersion(sipVersion)
	res.headers = newHeaders(hdrs)
	res.SetStatusCode(statusCode)
	res.SetReason(reason)

	if len(body) != 0 {
		res.SetBody(body, true)
	}

	return res
}

// Reason Reason
func (res *Response) Reason() string {
	return res.reason
}

// SetReason SetReason
func (res *Response) SetReason(reason string) {
	res.reason = reason
}

// SetStatusCode SetStatusCode
func (res *Response) SetStatusCode(code int) {
	res.statusCode = code
}

// StatusCode StatusCode
func (res *Response) StatusCode() int {
	return res.statusCode
}

// StartLine returns Response Status Line - RFC 2361 7.2.
func (res *Response) StartLine() string {
	var buffer bytes.Buffer

	// Every SIP response starts with a Status Line - RFC 2361 7.2.
	buffer.WriteString(
		fmt.Sprintf(
			"%s %d %s",
			res.SipVersion(),
			res.StatusCode(),
			res.Reason(),
		),
	)

	return buffer.String()
}

// Clone Clone
func (res *Response) Clone() Message {
	return NewResponse(
		"",
		res.SipVersion(),
		res.StatusCode(),
		res.Reason(),
		res.headers.CloneHeaders(),
		res.Body(),
	)
}

// IsAck IsAck
func (res *Response) IsAck() bool {
	if cseq, ok := res.CSeq(); ok {
		return cseq.MethodName == MethodACK
	}
	return false
}

// IsCancel IsCancel
func (res *Response) IsCancel() bool {
	if cseq, ok := res.CSeq(); ok {
		return cseq.MethodName == MethodCancel
	}
	return false
}
