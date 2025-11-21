package sip

import (
	"bytes"
	"fmt"
	"strings"
)

// HeadersBuilder HeadersBuilder
type HeadersBuilder struct {
	protocol        string
	protocolVersion string
	host            string
	transport       string

	contentType *ContentType
	method      string
	from        *FromHeader
	to          *ToHeader
	contact     *ContactHeader
	via         ViaHeader
	cseq        *CSeq
	callID      *CallID
	generic     map[string]Header
	userAgent   *UserAgentHeader
	maxForwards *MaxForwards
	allow       *AllowHeader
	supported   *SupportedHeader
	XGBVer      *XGBVer
	// recipient *URI
}

// NewHeaderBuilder NewHeaderBuilder
func NewHeaderBuilder() *HeadersBuilder {
	callID := CallID(RandString(32))
	maxForwards := MaxForwards(70)
	userAgent := UserAgentHeader("GoWVP")
	xgbVer := XGBVer("3.0")
	return &HeadersBuilder{
		protocol:        "SIP",
		protocolVersion: "2.0",
		host:            "localhost",
		transport:       "UDP",
		cseq:            &CSeq{SeqNo: 1},
		callID:          &callID,
		via:             make(ViaHeader, 0),
		userAgent:       &userAgent,
		maxForwards:     &maxForwards,
		generic:         make(map[string]Header),
		allow:           defaultAllowMethods,
		supported:       &SupportedHeader{Options: []string{}},
		XGBVer:          &xgbVer,
	}
}

// Build Build
func (hb *HeadersBuilder) Build() []Header {
	hdrs := make([]Header, 0)
	if hb.supported != nil {
		hdrs = append(hdrs, hb.supported)
	}
	if hb.allow != nil {
		hdrs = append(hdrs, hb.allow)
	}
	// if hb.route != nil {
	// 	hdrs = append(hdrs, hb.route)
	// }
	if len(hb.via) != 0 {
		via := make(ViaHeader, 0)
		via = append(via, hb.via...)
		hdrs = append(hdrs, via)
	}

	hdrs = append(hdrs, hb.cseq, hb.from, hb.to, hb.callID)

	if hb.contact != nil {
		hdrs = append(hdrs, hb.contact)
	}
	if hb.maxForwards != nil {
		hdrs = append(hdrs, hb.maxForwards)
	}
	// if hb.expires != nil {
	// 	hdrs = append(hdrs, hb.expires)
	// }

	// if hb.accept != nil {
	// 	hdrs = append(hdrs, hb.accept)
	// }
	if hb.userAgent != nil {
		hdrs = append(hdrs, hb.userAgent)
	}
	if hb.contentType != nil {
		hdrs = append(hdrs, hb.contentType)
	}
	if hb.XGBVer != nil {
		hdrs = append(hdrs, hb.XGBVer)
	}

	// for _, header := range hb.generic {
	// 	hdrs = append(hdrs, header)
	// }
	return hdrs
}

func (hb *HeadersBuilder) SetXGBVer() *HeadersBuilder {
	s := XGBVer("3.0")
	hb.XGBVer = &s
	return hb
}

// SetMethod SetMethod
func (hb *HeadersBuilder) SetMethod(method string) *HeadersBuilder {
	hb.method = method
	hb.cseq.MethodName = method

	return hb
}

// SetSeqNo SetSeqNo
func (hb *HeadersBuilder) SetSeqNo(seqNo uint) *HeadersBuilder {
	hb.cseq.SeqNo = uint32(seqNo)
	return hb
}

// SetFrom FromHeader
func (hb *HeadersBuilder) SetFrom(address *Address) *HeadersBuilder {
	address = address.Clone()
	if address.URI.Host() == "" {
		address.URI.SetHost(hb.host)
	}
	if _, ok := address.Params.Get("tag"); !ok {
		address.Params.Add("tag", String{Str: RandString(32)})
	}

	hb.from = &FromHeader{
		DisplayName: address.DisplayName,
		Address:     address.URI,
		Params:      address.Params,
	}

	return hb
}

// SetTo ToHeader
func (hb *HeadersBuilder) SetTo(address *Address) *HeadersBuilder {
	// TODO: 防止崩溃，但应该在上层，防止传递空指针
	if address == nil {
		return hb
	}
	address = address.Clone()
	if address.URI.Host() == "" {
		address.URI.SetHost(hb.host)
	}
	hb.to = &ToHeader{
		DisplayName: address.DisplayName,
		Address:     address.URI,
		// Params:      address.Params,
	}
	return hb
}

// SetTo ToHeader
func (hb *HeadersBuilder) SetToWithParam(address *Address) *HeadersBuilder {
	address = address.Clone()
	if address.URI.Host() == "" {
		address.URI.SetHost(hb.host)
	}
	hb.to = &ToHeader{
		DisplayName: address.DisplayName,
		Address:     address.URI,
		Params:      address.Params,
	}
	return hb
}

// SetContact SetContact
func (hb *HeadersBuilder) SetContact(address *Address) *HeadersBuilder {
	address = address.Clone()
	if address.URI.Host() == "" {
		address.URI.SetHost(hb.host)
	}

	hb.contact = &ContactHeader{
		DisplayName: address.DisplayName,
		Address:     address.URI,
		Params:      address.Params,
	}

	return hb
}

// AddVia AddVia
func (hb *HeadersBuilder) AddVia(via *ViaHop) *HeadersBuilder {
	if via.ProtocolName == "" {
		via.ProtocolName = hb.protocol
	}
	if via.ProtocolVersion == "" {
		via.ProtocolVersion = hb.protocolVersion
	}
	if via.Transport == "" {
		via.Transport = hb.transport
	}
	if via.Host == "" {
		via.Host = hb.host
	}
	if via.Params == nil {
		via.Params = NewParams()
	}

	hb.via = append(hb.via, via)

	return hb
}

// SetContentType SetContentType
func (hb *HeadersBuilder) SetContentType(contentType *ContentType) *HeadersBuilder {
	hb.contentType = contentType
	return hb
}

// SetCallID SetCallID
func (hb *HeadersBuilder) SetCallID(callID *CallID) *HeadersBuilder {
	if callID != nil {
		hb.callID = callID
	}

	return hb
}

// Params Generic list of parameters on a header.
type Params interface {
	Get(key string) (MaybeString, bool)
	Add(key string, val MaybeString) Params
	Clone() Params
	Equals(params interface{}) bool
	ToString(sep uint8) string
	String() string
	Length() int
	Items() map[string]MaybeString
	Keys() []string
	Has(key string) bool
}

// Address Address
type Address struct {
	DisplayName MaybeString
	URI         *URI
	Params      Params
}

func (addr Address) String() string {
	return fmt.Sprintf("%s %s", addr.URI.String(), addr.Params.String())
}

// Clone Clone
func (addr *Address) Clone() *Address {
	var name MaybeString
	var uri *URI
	var params Params

	if addr.DisplayName != nil {
		name = String{Str: addr.DisplayName.String()}
	}
	if addr.URI != nil {
		uri = addr.URI.Clone()
	}
	if addr.Params != nil {
		params = addr.Params.Clone()
	}

	return &Address{
		DisplayName: name,
		URI:         uri,
		Params:      params,
	}
}

// NewAddressFromFromHeader NewAddressFromFromHeader
func NewAddressFromFromHeader(from *FromHeader) *Address {
	addr := &Address{
		DisplayName: from.DisplayName,
	}
	if from.Address != nil {
		addr.URI = from.Address.Clone()
	}
	if from.Params != nil {
		addr.Params = from.Params.Clone()
	}

	return addr
}

// Header is a single SIP header.
type Header interface {
	// Name returns header name.
	Name() string
	// Clone returns copy of header struct.
	Clone() Header
	String() string
	Equals(other interface{}) bool
}

// headers is a struct with methods to work with SIP headers.
type headers struct {
	// The logical SIP headers attached to this message.
	headers map[string][]Header
	// The order the headers should be displayed in.
	headerOrder []string
}

// CopyHeaders Copy all headers of one type from one message to another.
// Appending to any headers that were already there.
func CopyHeaders(name string, from, to Message) {
	name = strings.ToLower(name)
	for _, h := range from.GetHeaders(name) {
		to.AppendHeader(h.Clone())
	}
}

func newHeaders(hdrs []Header) *headers {
	hs := new(headers)
	hs.headers = make(map[string][]Header)
	hs.headerOrder = make([]string, 0)
	for _, header := range hdrs {
		hs.AppendHeader(header)
	}
	return hs
}

// Via Via
func (hs *headers) Via() (ViaHeader, bool) {
	hdrs := hs.GetHeaders("Via")
	if len(hdrs) == 0 {
		return nil, false
	}
	via, ok := (hdrs[0]).(ViaHeader)
	if !ok {
		return nil, false
	}

	return via, true
}

// ViaHop ViaHop
func (hs *headers) ViaHop() (*ViaHop, bool) {
	via, ok := hs.Via()
	if !ok {
		return nil, false
	}
	hops := []*ViaHop(via)
	if len(hops) == 0 {
		return nil, false
	}

	return hops[0], true
}

func (hs *headers) CallID() (*CallID, bool) {
	hdrs := hs.GetHeaders("Call-ID")
	if len(hdrs) == 0 {
		return nil, false
	}
	callID, ok := hdrs[0].(*CallID)
	if !ok {
		return nil, false
	}
	return callID, true
}

// CSeq  CSeq
func (hs *headers) CSeq() (*CSeq, bool) {
	hdrs := hs.GetHeaders("CSeq")
	if len(hdrs) == 0 {
		return nil, false
	}
	cseq, ok := hdrs[0].(*CSeq)
	if !ok {
		return nil, false
	}
	return cseq, true
}

// AppendHeader Add the given header.
func (hs *headers) AppendHeader(header Header) {
	name := strings.ToLower(header.Name())
	if _, ok := hs.headers[name]; ok {
		hs.headers[name] = append(hs.headers[name], header)
	} else {
		hs.headers[name] = []Header{header}
		hs.headerOrder = append(hs.headerOrder, name)
	}
}

func (hs headers) String() string {
	buffer := bytes.Buffer{}
	// Construct each header in turn and add it to the message.
	for typeIdx, name := range hs.headerOrder {
		headers := hs.headers[name]
		for idx, header := range headers {
			buffer.WriteString(header.String())
			if typeIdx < len(hs.headerOrder) || idx < len(headers) {
				buffer.WriteString("\r\n")
			}
		}
	}
	return buffer.String()
}

func (hs *headers) GetHeaders(name string) []Header {
	name = strings.ToLower(name)
	if hs.headers == nil {
		hs.headers = map[string][]Header{}
		hs.headerOrder = []string{}
	}
	if headers, ok := hs.headers[name]; ok {
		return headers
	}

	return []Header{}
}

func (hs *headers) Contact() (*ContactHeader, bool) {
	hdrs := hs.GetHeaders("Contact")
	if len(hdrs) == 0 {
		return nil, false
	}
	contactHeader, ok := hdrs[0].(*ContactHeader)
	if !ok {
		return nil, false
	}
	return contactHeader, true
}

func (hs *headers) ContentLength() (*ContentLength, bool) {
	hdrs := hs.GetHeaders("Content-Length")
	if len(hdrs) == 0 {
		return nil, false
	}
	contentLength, ok := hdrs[0].(*ContentLength)
	if !ok {
		return nil, false
	}
	return contentLength, true
}

func (hs *headers) ContentType() (*ContentType, bool) {
	hdrs := hs.GetHeaders("Content-Type")
	if len(hdrs) == 0 {
		return nil, false
	}
	contentType, ok := hdrs[0].(*ContentType)
	if !ok {
		return nil, false
	}
	return contentType, true
}

func (hs *headers) From() (*FromHeader, bool) {
	hdrs := hs.GetHeaders("From")
	if len(hdrs) == 0 {
		return nil, false
	}
	from, ok := hdrs[0].(*FromHeader)
	if !ok {
		return nil, false
	}
	return from, true
}

func (hs *headers) To() (*ToHeader, bool) {
	hdrs := hs.GetHeaders("To")
	if len(hdrs) == 0 {
		return nil, false
	}
	to, ok := hdrs[0].(*ToHeader)
	if !ok {
		return nil, false
	}
	return to, true
}

// Gets some headers.
func (hs *headers) Headers() []Header {
	hdrs := make([]Header, 0)
	for _, key := range hs.headerOrder {
		hdrs = append(hdrs, hs.headers[key]...)
	}

	return hdrs
}

func (hs *headers) RemoveHeader(name string) {
	name = strings.ToLower(name)
	delete(hs.headers, name)
	// update order slice
	for idx, entry := range hs.headerOrder {
		if entry == name {
			hs.headerOrder = append(hs.headerOrder[:idx], hs.headerOrder[idx+1:]...)
			break
		}
	}
}

// CloneHeaders returns all cloned headers in slice.
func (hs *headers) CloneHeaders() []Header {
	hdrs := make([]Header, 0)
	for _, header := range hs.Headers() {
		hdrs = append(hdrs, header.Clone())
	}

	return hdrs
}

// Params implementation.
type headerParams struct {
	params     map[string]MaybeString
	paramOrder []string
}

// NewParams Create an empty set of parameters.
func NewParams() Params {
	return &headerParams{
		params:     make(map[string]MaybeString),
		paramOrder: []string{},
	}
}

// Returns the entire parameter map.
func (params *headerParams) Items() map[string]MaybeString {
	return params.params
}

// Returns a slice of keys, in order.
func (params *headerParams) Keys() []string {
	return params.paramOrder
}

// Returns the requested parameter value.
func (params *headerParams) Get(key string) (MaybeString, bool) {
	v, ok := params.params[key]
	return v, ok
}

// Put a new parameter.
func (params *headerParams) Add(key string, val MaybeString) Params {
	// Add param to order list if new.
	if _, ok := params.params[key]; !ok {
		params.paramOrder = append(params.paramOrder, key)
	}

	// Set param value.
	params.params[key] = val

	// Return the params so calls can be chained.
	return params
}

func (params *headerParams) Has(key string) bool {
	_, ok := params.params[key]

	return ok
}

// Copy a list of params.
func (params *headerParams) Clone() Params {
	if params == nil {
		var dup *headerParams
		return dup
	}

	dup := NewParams()
	for _, key := range params.Keys() {
		if val, ok := params.Get(key); ok {
			dup.Add(key, val)
		}
	}

	return dup
}

// Render params to a string.
// Note that this does not escape special characters, this should already have been done before calling this method.
func (params *headerParams) ToString(sep uint8) string {
	if params == nil {
		return ""
	}

	var buffer bytes.Buffer
	first := true

	for _, key := range params.Keys() {
		val, ok := params.Get(key)
		if !ok {
			continue
		}

		if !first {
			buffer.WriteString(fmt.Sprintf("%c", sep))
		}
		first = false

		buffer.WriteString(key)

		if val, ok := val.(String); ok {
			if strings.ContainsAny(val.String(), abnfWs) {
				buffer.WriteString(fmt.Sprintf("=\"%s\"", val.String()))
			} else {
				buffer.WriteString(fmt.Sprintf("=%s", val.String()))
			}
		}
	}

	return buffer.String()
}

// String returns params joined with '&' char.
func (params *headerParams) String() string {
	if params == nil {
		return ""
	}

	return params.ToString('&')
}

// Returns number of params.
func (params *headerParams) Length() int {
	return len(params.params)
}

// Check if two maps of parameters are equal in the sense of having the same keys with the same values.
// This does not rely on any ordering of the keys of the map in memory.
func (params *headerParams) Equals(other interface{}) bool {
	q, ok := other.(*headerParams)
	if !ok {
		return false
	}

	if params == q {
		return true
	}
	if params == nil && q != nil || params != nil && q == nil {
		return false
	}

	if params.Length() == 0 && q.Length() == 0 {
		return true
	}

	if params.Length() != q.Length() {
		return false
	}

	for key, pVal := range params.Items() {
		qVal, ok := q.Get(key)
		if !ok {
			return false
		}
		if pVal != qVal {
			return false
		}
	}

	return true
}

// ==================   ContentLengthHeader   ================

// ContentLength ContentLength header
type ContentLength uint32

func (contentLength ContentLength) String() string {
	return fmt.Sprintf("Content-Length: %d", int(contentLength))
}

// Name Name
func (contentLength *ContentLength) Name() string { return "Content-Length" }

// Clone Clone
func (contentLength *ContentLength) Clone() Header { return contentLength }

// Equals Equals
func (contentLength *ContentLength) Equals(other interface{}) bool {
	if h, ok := other.(ContentLength); ok {
		if contentLength == nil {
			return false
		}

		return *contentLength == h
	}
	if h, ok := other.(*ContentLength); ok {
		if contentLength == h {
			return true
		}
		if contentLength == nil && h != nil || contentLength != nil && h == nil {
			return false
		}

		return *contentLength == *h
	}

	return false
}

// ==================   viaHeader   ================

// ViaHeader ViaHeader
type ViaHeader []*ViaHop

func (via ViaHeader) String() string {
	var buffer bytes.Buffer
	buffer.WriteString("Via: ")
	for idx, hop := range via {
		buffer.WriteString(hop.String())
		if idx != len(via)-1 {
			buffer.WriteString(", ")
		}
	}

	return buffer.String()
}

// Name Name
func (via ViaHeader) Name() string { return "Via" }

// Clone Clone
func (via ViaHeader) Clone() Header {
	if via == nil {
		var newVie ViaHeader
		return newVie
	}

	dup := make([]*ViaHop, 0, len(via))
	for _, hop := range via {
		dup = append(dup, hop.Clone())
	}
	return ViaHeader(dup)
}

// Equals Equals
func (via ViaHeader) Equals(other interface{}) bool {
	if h, ok := other.(ViaHeader); ok {
		if len(via) != len(h) {
			return false
		}

		for i, hop := range via {
			if !hop.Equals(h[i]) {
				return false
			}
		}

		return true
	}

	return false
}

// ViaHop A single component in a Via header.
// Via headers are composed of several segments of the same structure, added by successive nodes in a routing chain.
type ViaHop struct {
	// E.g. 'SIP'.
	ProtocolName string
	// E.g. '2.0'.
	ProtocolVersion string
	Transport       string
	Host            string
	// The port for this via hop. This is stored as a pointer type, since it is an optional field.
	Port   *Port
	Params Params
}

// SentBy SentBy
func (hop *ViaHop) SentBy() string {
	var buf bytes.Buffer
	buf.WriteString(hop.Host)
	if hop.Port != nil {
		buf.WriteString(fmt.Sprintf(":%d", *hop.Port))
	}

	return buf.String()
}

func (hop *ViaHop) String() string {
	var buffer bytes.Buffer
	buffer.WriteString(
		fmt.Sprintf(
			"%s/%s/%s %s",
			hop.ProtocolName,
			hop.ProtocolVersion,
			hop.Transport,
			hop.Host,
		),
	)
	if hop.Port != nil {
		buffer.WriteString(fmt.Sprintf(":%d", *hop.Port))
	}

	if hop.Params.Length() > 0 {
		buffer.WriteString(";")
		buffer.WriteString(hop.Params.ToString(';'))
	}

	return buffer.String()
}

// Clone Return an exact copy of this ViaHop.
func (hop *ViaHop) Clone() *ViaHop {
	var newHop *ViaHop
	if hop == nil {
		return newHop
	}

	newHop = &ViaHop{
		ProtocolName:    hop.ProtocolName,
		ProtocolVersion: hop.ProtocolVersion,
		Transport:       hop.Transport,
		Host:            hop.Host,
	}
	if hop.Port != nil {
		newHop.Port = hop.Port.Clone()
	}
	if hop.Params != nil {
		newHop.Params = hop.Params.Clone()
	}

	return newHop
}

// Equals Equals
func (hop *ViaHop) Equals(other interface{}) bool {
	if h, ok := other.(*ViaHop); ok {
		if hop == h {
			return true
		}
		if hop == nil && h != nil || hop != nil && h == nil {
			return false
		}

		res := hop.ProtocolName == h.ProtocolName &&
			hop.ProtocolVersion == h.ProtocolVersion &&
			hop.Transport == h.Transport &&
			hop.Host == h.Host &&
			Uint16PtrEq((*uint16)(hop.Port), (*uint16)(h.Port))

		if hop.Params != h.Params {
			if hop.Params == nil {
				res = res && h.Params == nil
			} else {
				res = res && hop.Params.Equals(h.Params)
			}
		}

		return res
	}

	return false
}

// ==================   callidHeader   ================

// CallID - 'Call-ID' header.
type CallID string

func (callId CallID) String() string {
	return "Call-ID: " + string(callId)
}

// Name Name
func (callId *CallID) Name() string { return "Call-ID" }

// Clone Clone
func (callId *CallID) Clone() Header {
	return callId
}

// Equals Equals
func (callId *CallID) Equals(other interface{}) bool {
	if h, ok := other.(CallID); ok {
		if callId == nil {
			return false
		}

		return *callId == h
	}
	if h, ok := other.(*CallID); ok {
		if callId == h {
			return true
		}
		if callId == nil && h != nil || callId != nil && h == nil {
			return false
		}

		return *callId == *h
	}

	return false
}

// ==================   CSeqHeader   ================

// CSeq CSeq
type CSeq struct {
	SeqNo      uint32
	MethodName string
}

func (cseq *CSeq) String() string {
	return fmt.Sprintf("CSeq: %d %s", cseq.SeqNo, cseq.MethodName)
}

// Name Name
func (cseq *CSeq) Name() string { return "CSeq" }

// Clone Clone
func (cseq *CSeq) Clone() Header {
	if cseq == nil {
		var newCSeq *CSeq
		return newCSeq
	}

	return &CSeq{
		SeqNo:      cseq.SeqNo,
		MethodName: cseq.MethodName,
	}
}

// Equals Equals
func (cseq *CSeq) Equals(other interface{}) bool {
	if h, ok := other.(*CSeq); ok {
		if cseq == h {
			return true
		}
		if cseq == nil && h != nil || cseq != nil && h == nil {
			return false
		}

		return cseq.SeqNo == h.SeqNo &&
			cseq.MethodName == h.MethodName
	}

	return false
}

// ==================   toHeader   ================

// ToHeader introduces SIP 'To' header
type ToHeader struct {
	// The display name from the header, may be omitted.
	DisplayName MaybeString
	Address     *URI
	// Any parameters present in the header.
	Params Params
}

func (to *ToHeader) String() string {
	var buffer bytes.Buffer
	buffer.WriteString("To: ")

	if displayName, ok := to.DisplayName.(String); ok && displayName.String() != "" {
		buffer.WriteString(fmt.Sprintf("\"%s\" ", displayName))
	}

	buffer.WriteString(fmt.Sprintf("<%s>", to.Address))

	if to.Params != nil && to.Params.Length() > 0 {
		buffer.WriteString(";")
		buffer.WriteString(to.Params.ToString(';'))
	}

	return buffer.String()
}

// Name Name
func (to *ToHeader) Name() string { return "To" }

// Clone Copy the header.
func (to *ToHeader) Clone() Header {
	var newTo *ToHeader
	if to == nil {
		return newTo
	}

	newTo = &ToHeader{
		DisplayName: to.DisplayName,
	}
	newTo.Address = to.Address.Clone()
	if to.Params != nil {
		newTo.Params = to.Params.Clone()
	}
	return newTo
}

// Equals Equals
func (to *ToHeader) Equals(other interface{}) bool {
	if h, ok := other.(*ToHeader); ok {
		if to == h {
			return true
		}
		if to == nil && h != nil || to != nil && h == nil {
			return false
		}

		res := true

		if to.DisplayName != h.DisplayName {
			if to.DisplayName == nil {
				res = res && h.DisplayName == nil
			} else {
				res = res && to.DisplayName.Equals(h.DisplayName)
			}
		}

		if to.Address != h.Address {
			if to.Address == nil {
				res = res && h.Address == nil
			} else {
				res = res && to.Address.Equals(h.Address)
			}
		}

		if to.Params != h.Params {
			if to.Params == nil {
				res = res && h.Params == nil
			} else {
				res = res && to.Params.Equals(h.Params)
			}
		}

		return res
	}

	return false
}

// ==================   fromHeader   ================

// FromHeader FromHeader
type FromHeader struct {
	// The display name from the header, may be omitted.
	DisplayName MaybeString

	Address *URI

	// Any parameters present in the header.
	Params Params
}

func (from *FromHeader) String() string {
	var buffer bytes.Buffer
	buffer.WriteString("From: ")

	if displayName, ok := from.DisplayName.(String); ok && displayName.String() != "" {
		buffer.WriteString(fmt.Sprintf("\"%s\" ", displayName))
	}

	buffer.WriteString(fmt.Sprintf("<%s>", from.Address))

	if from.Params.Length() > 0 {
		buffer.WriteString(";")
		buffer.WriteString(from.Params.ToString(';'))
	}

	return buffer.String()
}

// Name Name
func (from *FromHeader) Name() string { return "From" }

// Clone Copy the header.
func (from *FromHeader) Clone() Header {
	var newFrom *FromHeader
	if from == nil {
		return newFrom
	}

	newFrom = &FromHeader{
		DisplayName: from.DisplayName,
	}
	if from.Address != nil {
		newFrom.Address = from.Address.Clone()
	}
	if from.Params != nil {
		newFrom.Params = from.Params.Clone()
	}

	return newFrom
}

// Equals Equals
func (from *FromHeader) Equals(other interface{}) bool {
	if h, ok := other.(*FromHeader); ok {
		if from == h {
			return true
		}
		if from == nil && h != nil || from != nil && h == nil {
			return false
		}

		res := true

		if from.DisplayName != h.DisplayName {
			if from.DisplayName == nil {
				res = res && h.DisplayName == nil
			} else {
				res = res && from.DisplayName.Equals(h.DisplayName)
			}
		}

		if from.Address != h.Address {
			if from.Address == nil {
				res = res && h.Address == nil
			} else {
				res = res && from.Address.Equals(h.Address)
			}
		}

		if from.Params != h.Params {
			if from.Params == nil {
				res = res && h.Params == nil
			} else {
				res = res && from.Params.Equals(h.Params)
			}
		}

		return res
	}

	return false
}

// ==================   ContentTypeHeader   ================

// ContentType ContentType
type ContentType string

func (ct ContentType) String() string { return "Content-Type: " + string(ct) }

// Name Name
func (ct *ContentType) Name() string { return "Content-Type" }

// Clone Clone
func (ct *ContentType) Clone() Header { return ct }

// Equals Equals
func (ct *ContentType) Equals(other interface{}) bool {
	if h, ok := other.(ContentType); ok {
		if ct == nil {
			return false
		}

		return *ct == h
	}
	if h, ok := other.(*ContentType); ok {
		if ct == h {
			return true
		}
		if ct == nil && h != nil || ct != nil && h == nil {
			return false
		}

		return *ct == *h
	}

	return false
}

// ==================   ContactHeaderHeader   ================

// ContactHeader ContactHeader
type ContactHeader struct {
	// The display name from the header, may be omitted.
	DisplayName MaybeString
	Address     *URI
	// Any parameters present in the header.
	Params Params
}

func (contact *ContactHeader) String() string {
	var buffer bytes.Buffer
	buffer.WriteString("Contact: ")

	if displayName, ok := contact.DisplayName.(String); ok && displayName.String() != "" {
		buffer.WriteString(fmt.Sprintf("\"%s\" ", displayName))
	}

	buffer.WriteString(fmt.Sprintf("<%s>", contact.Address.String()))

	if (contact.Params != nil) && (contact.Params.Length() > 0) {
		buffer.WriteString(";")
		buffer.WriteString(contact.Params.ToString(';'))
	}

	return buffer.String()
}

// Name Name
func (contact *ContactHeader) Name() string { return "Contact" }

// Clone Copy the header.
func (contact *ContactHeader) Clone() Header {
	var newCnt *ContactHeader
	if contact == nil {
		return newCnt
	}

	newCnt = &ContactHeader{
		DisplayName: contact.DisplayName,
	}
	newCnt.Address = contact.Address.Clone()
	if contact.Params != nil {
		newCnt.Params = contact.Params.Clone()
	}

	return newCnt
}

// Equals Equals
func (contact *ContactHeader) Equals(other interface{}) bool {
	if h, ok := other.(*ContactHeader); ok {
		if contact == h {
			return true
		}
		if contact == nil && h != nil || contact != nil && h == nil {
			return false
		}

		res := true

		if contact.DisplayName != h.DisplayName {
			if contact.DisplayName == nil {
				res = res && h.DisplayName == nil
			} else {
				res = res && contact.DisplayName.Equals(h.DisplayName)
			}
		}

		if contact.Address != h.Address {
			if contact.Address == nil {
				res = res && h.Address == nil
			} else {
				res = res && contact.Address.Equals(h.Address)
			}
		}

		if contact.Params != h.Params {
			if contact.Params == nil {
				res = res && h.Params == nil
			} else {
				res = res && contact.Params.Equals(h.Params)
			}
		}

		return res
	}

	return false
}

// ==================   MaxForwardsHeader   ================

// MaxForwards MaxForwards
type MaxForwards uint32

func (maxForwards MaxForwards) String() string {
	return fmt.Sprintf("Max-Forwards: %d", int(maxForwards))
}

// Name Name
func (maxForwards *MaxForwards) Name() string { return "Max-Forwards" }

// Clone Clone
func (maxForwards *MaxForwards) Clone() Header { return maxForwards }

// Equals Equals
func (maxForwards *MaxForwards) Equals(other interface{}) bool {
	if h, ok := other.(MaxForwards); ok {
		if maxForwards == nil {
			return false
		}

		return *maxForwards == h
	}
	if h, ok := other.(*MaxForwards); ok {
		if maxForwards == h {
			return true
		}
		if maxForwards == nil && h != nil || maxForwards != nil && h == nil {
			return false
		}

		return *maxForwards == *h
	}

	return false
}

// ==================   ExpiresHeader   ================

// Expires Expires
type Expires uint32

func (expires Expires) String() string {
	return fmt.Sprintf("Expires: %d", int(expires))
}

// Name Name
func (expires *Expires) Name() string { return "Expires" }

// Clone clone
func (expires *Expires) Clone() Header { return expires }

// Equals Equals
func (expires *Expires) Equals(other interface{}) bool {
	if h, ok := other.(Expires); ok {
		if expires == nil {
			return false
		}

		return *expires == h
	}
	if h, ok := other.(*Expires); ok {
		if expires == h {
			return true
		}
		if expires == nil && h != nil || expires != nil && h == nil {
			return false
		}

		return *expires == *h
	}

	return false
}

// ==================   UserAgentHeaderHeader   ================

// UserAgentHeader UserAgentHeader
type UserAgentHeader string

func (ua UserAgentHeader) String() string {
	return "User-Agent: " + string(ua)
}

// Name Name
func (ua *UserAgentHeader) Name() string { return "User-Agent" }

// Clone clone
func (ua *UserAgentHeader) Clone() Header { return ua }

// Equals equals
func (ua *UserAgentHeader) Equals(other interface{}) bool {
	if h, ok := other.(UserAgentHeader); ok {
		if ua == nil {
			return false
		}

		return *ua == h
	}
	if h, ok := other.(*UserAgentHeader); ok {
		if ua == h {
			return true
		}
		if ua == nil && h != nil || ua != nil && h == nil {
			return false
		}

		return *ua == *h
	}

	return false
}

// ==================   AllowHeader   ================

var defaultAllowMethods = &AllowHeader{MethodInvite, MethodACK, MethodCancel, MethodMessage, MethodRegister}

// AllowHeader AllowHeader
type AllowHeader []string

func (allow AllowHeader) String() string {
	parts := make([]string, 0)
	for _, method := range allow {
		parts = append(parts, string(method))
	}

	return fmt.Sprintf("Allow: %s", strings.Join(parts, ", "))
}

// Name Name
func (allow AllowHeader) Name() string { return "Allow" }

// Clone Clone
func (allow AllowHeader) Clone() Header {
	if allow == nil {
		var newAllow AllowHeader
		return newAllow
	}

	newAllow := make(AllowHeader, len(allow))
	copy(newAllow, allow)

	return newAllow
}

// Equals equals
func (allow AllowHeader) Equals(other interface{}) bool {
	if h, ok := other.(AllowHeader); ok {
		if len(allow) != len(h) {
			return false
		}

		for i, v := range allow {
			if v != h[i] {
				return false
			}
		}

		return true
	}

	return false
}

// ==================   Accept   ================

// Accept Accept
type Accept string

func (ct Accept) String() string { return "Accept: " + string(ct) }

// Name Name
func (ct *Accept) Name() string { return "Accept" }

// Clone Clone
func (ct *Accept) Clone() Header { return ct }

// Equals Equals
func (ct *Accept) Equals(other interface{}) bool {
	if h, ok := other.(Accept); ok {
		if ct == nil {
			return false
		}

		return *ct == h
	}
	if h, ok := other.(*Accept); ok {
		if ct == h {
			return true
		}
		if ct == nil && h != nil || ct != nil && h == nil {
			return false
		}

		return *ct == *h
	}

	return false
}

// ==================   RouteHeader   ================

// RouteHeader RouteHeader
type RouteHeader struct {
	Addresses []*URI
}

// Name  Name
func (route *RouteHeader) Name() string { return "Route" }

func (route *RouteHeader) String() string {
	var addrs []string

	for _, uri := range route.Addresses {
		addrs = append(addrs, "<"+uri.String()+">")
	}

	return fmt.Sprintf("Route: %s", strings.Join(addrs, ", "))
}

// Clone Clone
func (route *RouteHeader) Clone() Header {
	var newRoute *RouteHeader
	if route == nil {
		return newRoute
	}

	newRoute = &RouteHeader{
		Addresses: []*URI{},
	}

	for i, uri := range route.Addresses {
		newRoute.Addresses[i] = uri.Clone()
	}

	return newRoute
}

// Equals Equals
func (route *RouteHeader) Equals(other interface{}) bool {
	if h, ok := other.(*RouteHeader); ok {
		if route == h {
			return true
		}
		if route == nil && h != nil || route != nil && h == nil {
			return false
		}

		for i, uri := range route.Addresses {
			if !uri.Equals(h.Addresses[i]) {
				return false
			}
		}

		return true
	}

	return false
}

// ==================   RecordRouteHeader   ================

// RecordRouteHeader RecordRouteHeader
type RecordRouteHeader struct {
	Addresses []*URI
}

// Name Name
func (route *RecordRouteHeader) Name() string { return "Record-Route" }

func (route *RecordRouteHeader) String() string {
	var addrs []string

	for _, uri := range route.Addresses {
		addrs = append(addrs, "<"+uri.String()+">")
	}

	return fmt.Sprintf("Record-Route: %s", strings.Join(addrs, ", "))
}

// Clone Clone
func (route *RecordRouteHeader) Clone() Header {
	var newRoute *RecordRouteHeader
	if route == nil {
		return newRoute
	}

	newRoute = &RecordRouteHeader{
		Addresses: make([]*URI, 0, len(route.Addresses)),
	}

	for i, uri := range route.Addresses {
		newRoute.Addresses[i] = uri.Clone()
	}

	return newRoute
}

// Equals Equals
func (route *RecordRouteHeader) Equals(other interface{}) bool {
	if h, ok := other.(*RecordRouteHeader); ok {
		if route == h {
			return true
		}
		if route == nil && h != nil || route != nil && h == nil {
			return false
		}

		for i, uri := range route.Addresses {
			if !uri.Equals(h.Addresses[i]) {
				return false
			}
		}

		return true
	}

	return false
}

// ==================   SupportedHeader   ================

// SupportedHeader SupportedHeader
type SupportedHeader struct {
	Options []string
}

func (support *SupportedHeader) String() string {
	return fmt.Sprintf("Supported: %s",
		strings.Join(support.Options, ", "))
}

// Name name
func (support *SupportedHeader) Name() string { return "Supported" }

// Clone clone
func (support *SupportedHeader) Clone() Header {
	if support == nil {
		var newSupport *SupportedHeader
		return newSupport
	}

	dup := make([]string, len(support.Options))
	copy(dup, support.Options)
	return &SupportedHeader{dup}
}

// Equals Equals
func (support *SupportedHeader) Equals(other interface{}) bool {
	if h, ok := other.(*SupportedHeader); ok {
		if support == h {
			return true
		}
		if support == nil && h != nil || support != nil && h == nil {
			return false
		}

		if len(support.Options) != len(h.Options) {
			return false
		}

		for i, opt := range support.Options {
			if opt != h.Options[i] {
				return false
			}
		}

		return true
	}

	return false
}

// GenericHeader Encapsulates a header that gossip does not natively support.
// This allows header data that is not understood to be parsed by gossip and relayed to the parent application.
type GenericHeader struct {
	// The name of the header.
	HeaderName string
	// The contents of the header, including any parameters.
	// This is transparent data that is not natively understood by gossip.
	Contents string
}

// Convert the header to a flat string representation.
func (header *GenericHeader) String() string {
	return header.HeaderName + ": " + header.Contents
}

// Name Pull out the header name.
func (header *GenericHeader) Name() string {
	return header.HeaderName
}

// Clone Copy the header.
func (header *GenericHeader) Clone() Header {
	if header == nil {
		var newHeader *GenericHeader
		return newHeader
	}

	return &GenericHeader{
		HeaderName: header.HeaderName,
		Contents:   header.Contents,
	}
}

// Equals Equals
func (header *GenericHeader) Equals(other interface{}) bool {
	if h, ok := other.(*GenericHeader); ok {
		if header == h {
			return true
		}
		if header == nil && h != nil || header != nil && h == nil {
			return false
		}

		return header.HeaderName == h.HeaderName &&
			header.Contents == h.Contents
	}

	return false
}

type XGBVer string

func (ct XGBVer) String() string { return "X-GB-Ver: " + string(ct) }

// Name Name
func (ct XGBVer) Name() string { return "X-GB-Ver" }

// Clone Clone
func (ct XGBVer) Clone() Header { return &ct }

// Equals Equals
func (ct *XGBVer) Equals(other interface{}) bool {
	h, ok := other.(XGBVer)
	if !ok {
		return false
	}
	return h.String() == ct.String()
}
