package sip

import (
	"bytes"
	"fmt"
	"net"

	"github.com/gofrs/uuid"
)

// Request Request
type Request struct {
	message
	method    string
	recipient *URI
}

// NewRequest NewRequest
func NewRequest(
	messID MessageID,
	method string,
	recipient *URI,
	sipVersion string,
	hdrs []Header,
	body []byte,
) *Request {
	req := new(Request)
	if messID == "" {
		req.messID = MessageID(uuid.Must(uuid.NewV4()).String())
	} else {
		req.messID = messID
	}
	req.SetSipVersion(sipVersion)
	req.startLine = req.StartLine
	req.headers = newHeaders(hdrs)

	req.SetMethod(method)
	req.SetRecipient(recipient)

	if len(body) != 0 {
		req.SetBody(body, true)
	}
	return req
}

// NewRequestFromResponse NewRequestFromResponse
func NewRequestFromResponse(method string, resp *Response) *Request {
	contact, _ := resp.Contact()
	ackRequest := NewRequest(
		resp.MessageID(),
		method,
		contact.Address,
		resp.SipVersion(),
		[]Header{},
		[]byte{},
	)

	CopyHeaders("Via", resp, ackRequest)
	viaHop, _ := ackRequest.ViaHop()
	// update branch, 2xx ACK is separate Tx
	viaHop.Params.Add("branch", String{Str: GenerateBranch()})

	if len(resp.GetHeaders("Route")) > 0 {
		CopyHeaders("Route", resp, ackRequest)
	} else {
		for _, h := range resp.GetHeaders("Record-Route") {
			uris := make([]*URI, 0)
			for _, u := range h.(*RecordRouteHeader).Addresses {
				uris = append(uris, u.Clone())
			}
			ackRequest.AppendHeader(&RouteHeader{
				Addresses: uris,
			})
		}
	}

	CopyHeaders("From", resp, ackRequest)
	CopyHeaders("To", resp, ackRequest)
	CopyHeaders("Call-ID", resp, ackRequest)
	cseq, _ := resp.CSeq()
	cseq.MethodName = method

	// https://www.rfc-editor.org/rfc/rfc3261.html#section-12.2.1.1
	// The Call-ID of the request MUST be set to the Call-ID of the dialog.
	// Requests within a dialog MUST contain strictly monotonically
	// increasing and contiguous CSeq sequence numbers (increasing-by-one)
	// in each direction (excepting ACK and CANCEL of course, whose numbers
	// equal the requests being acknowledged or cancelled).  Therefore, if
	// the local sequence number is not empty, the value of the local
	// sequence number MUST be incremented by one, and this value MUST be
	// placed into the CSeq header field.
	if !(method == MethodACK || method == MethodCancel) {
		cseq.SeqNo++
	}
	ackRequest.AppendHeader(cseq)
	ackRequest.SetSource(resp.Destination())
	ackRequest.SetDestination(resp.Source())
	return ackRequest
}

// StartLine returns Request Line - RFC 2361 7.1.
func (req *Request) StartLine() string {
	var buffer bytes.Buffer

	// Every SIP request starts with a Request Line - RFC 2361 7.1.
	buffer.WriteString(
		fmt.Sprintf(
			"%s %s %s",
			req.method,
			req.Recipient(),
			req.SipVersion(),
		),
	)

	return buffer.String()
}

// Method Method
func (req *Request) Method() string {
	return req.method
}

// SetMethod SetMethod
func (req *Request) SetMethod(method string) {
	req.method = method
}

// Recipient Recipient
func (req *Request) Recipient() *URI {
	return req.recipient
}

// SetRecipient SetRecipient
func (req *Request) SetRecipient(recipient *URI) {
	req.recipient = recipient
}

// IsInvite IsInvite
func (req *Request) IsInvite() bool {
	return req.Method() == MethodInvite
}

// IsAck IsAck
func (req *Request) IsAck() bool {
	return req.Method() == MethodACK
}

// IsCancel IsCancel
func (req *Request) IsCancel() bool {
	return req.Method() == MethodCancel
}

// Source Source
func (req *Request) Source() net.Addr {
	return req.source
}

// SetSource SetSource
func (req *Request) SetSource(src net.Addr) {
	req.source = src
}

// Destination Destination
func (req *Request) Destination() net.Addr {
	return req.dest
}

// SetDestination SetDestination
func (req *Request) SetDestination(dest net.Addr) {
	req.dest = dest
}

func (req *Request) SetConnection(conn Connection) {
	req.conn = conn
}

func (req *Request) GetConnection() Connection {
	return req.conn
}

// Clone Clone
func (req *Request) Clone() Message {
	return NewRequest(
		"",
		req.Method(),
		req.Recipient().Clone(),
		req.SipVersion(),
		req.headers.CloneHeaders(),
		req.Body(),
	)
}
