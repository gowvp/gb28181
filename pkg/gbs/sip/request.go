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

// NewRequestFromResponse 基于 SIP 响应构造同一对话内的后续请求（ACK/BYE/INFO 等）。
// 防御性处理：Response 可能来自异常设备，缺少 Contact/Via/CSeq 等必选头部时不 panic，
// 用 To 地址兜底或跳过对应字段，确保 logout 等清理路径不会因残缺响应导致进程崩溃。
func NewRequestFromResponse(method string, resp *Response) *Request {
	// Request-URI 优先取 Contact，缺失则回退 To（RFC 3261 §12.2.1.1）
	var recipient *URI
	if contact, ok := resp.Contact(); ok && contact != nil && contact.Address != nil {
		recipient = contact.Address
	} else if to, ok := resp.To(); ok && to != nil && to.Address != nil {
		recipient = to.Address
	}

	ackRequest := NewRequest(
		resp.MessageID(),
		method,
		recipient,
		resp.SipVersion(),
		[]Header{},
		[]byte{},
	)

	CopyHeaders("Via", resp, ackRequest)
	if viaHop, ok := ackRequest.ViaHop(); ok && viaHop != nil {
		// update branch, 2xx ACK is separate Tx
		viaHop.Params.Add("branch", String{Str: GenerateBranch()})
	}

	if len(resp.GetHeaders("Route")) > 0 {
		CopyHeaders("Route", resp, ackRequest)
	} else {
		for _, h := range resp.GetHeaders("Record-Route") {
			rr, ok := h.(*RecordRouteHeader)
			if !ok || rr == nil {
				continue
			}
			uris := make([]*URI, 0, len(rr.Addresses))
			for _, u := range rr.Addresses {
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
	if cseq, ok := resp.CSeq(); ok && cseq != nil {
		cseq.MethodName = method
		// https://www.rfc-editor.org/rfc/rfc3261.html#section-12.2.1.1
		if !(method == MethodACK || method == MethodCancel) {
			cseq.SeqNo++
		}
		ackRequest.AppendHeader(cseq)
	}
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
