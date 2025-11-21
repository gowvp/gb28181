package gbs

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gowvp/gb28181/pkg/gbs/sip"
)

// 获取录像文件列表
func SipRecordList(to *Channels, start, end int64) (*Records, error) {
	sn := sip.RandInt(100000, 999999)
	resp := make(chan Records, 1)
	defer close(resp)
	device, ok := _activeDevices.Get(to.DeviceID)
	if !ok {
		return nil, errors.New("设备不在线")
	}
	channelURI, _ := sip.ParseURI(to.URIStr)
	to.addr = &sip.Address{URI: channelURI}
	recordKey := fmt.Sprintf("%s%d", to.ChannelID, sn)
	_recordList.Store(recordKey, recordList{channelid: to.ChannelID, resp: resp, data: [][]int64{}, l: &sync.Mutex{}, s: start, e: end})
	defer _recordList.Delete(recordKey)
	hb := sip.NewHeaderBuilder().SetTo(to.addr).SetFrom(_serverDevices.addr).AddVia(&sip.ViaHop{
		Params: sip.NewParams().Add("branch", sip.String{Str: sip.GenerateBranch()}),
	}).SetContentType(&sip.ContentTypeXML).SetMethod(sip.MethodMessage)
	req := sip.NewRequest("", sip.MethodMessage, to.addr.URI, sip.DefaultSipVersion, hb.Build(), sip.GetRecordInfoXML(to.ChannelID, sn, start, end))
	req.SetDestination(device.source)
	tx, err := svr.Request(req)
	if err != nil {
		return nil, err
	}
	response := tx.GetResponse()
	if response.StatusCode() != http.StatusOK {
		return nil, errors.New(response.Reason())
	}
	tick := time.NewTicker(10 * time.Second)
	select {
	case res := <-resp:
		return &res, nil
	case <-tick.C:
		// 10秒未完成返回当前获取到的数据
		if list, ok := _recordList.Load(recordKey); ok {
			info := list.(recordList)
			data := transRecordList(info.data)
			return &data, nil
		}
		return nil, errors.New("获取数据超时")
	}
}

// MessageRecordInfoResponse 目录列表
type MessageRecordInfoResponse struct {
	CmdType  string       `xml:"CmdType"`
	SN       int          `xml:"SN"`
	DeviceID string       `xml:"DeviceID"`
	SumNum   int          `xml:"SumNum"`
	Item     []RecordItem `xml:"RecordList>Item"`
}

// RecordItem 目录详情
type RecordItem struct {
	// DeviceID 设备编号
	DeviceID string `xml:"DeviceID" bson:"DeviceID" json:"DeviceID"`
	// Name 设备名称
	Name      string `xml:"Name" bson:"Name" json:"Name"`
	FilePath  string `xml:"FilePath" bson:"FilePath" json:"FilePath"`
	Address   string `xml:"Address" bson:"Address" json:"Address"`
	StartTime string `xml:"StartTime" bson:"StartTime" json:"StartTime"`
	EndTime   string `xml:"EndTime" bson:"EndTime" json:"EndTime"`
	Secrecy   int    `xml:"Secrecy" bson:"Secrecy" json:"Secrecy"`
	Type      string `xml:"Type" bson:"Type" json:"Type"`
}

type recordList struct {
	channelid string
	resp      chan Records
	num       int
	data      [][]int64
	l         *sync.Mutex
	s, e      int64
}

// 当前获取目录文件设备集合
var _recordList *sync.Map

func sipMessageRecordInfo(u Devices, body []byte) error {
	message := &MessageRecordInfoResponse{}
	if err := sip.XMLDecode(body, message); err != nil {
		// logrus.Errorln("Message Unmarshal xml err:", err, "body:", string(body))
		return err
	}
	recordKey := fmt.Sprintf("%s%d", message.DeviceID, message.SN)
	if list, ok := _recordList.Load(recordKey); ok {
		info := list.(recordList)
		info.l.Lock()
		defer info.l.Unlock()
		info.num += len(message.Item)
		var sint, eint int64
		for _, item := range message.Item {
			s, _ := time.ParseInLocation("2006-01-02T15:04:05", item.StartTime, time.Local)
			e, _ := time.ParseInLocation("2006-01-02T15:04:05", item.EndTime, time.Local)
			sint = s.Unix()
			eint = e.Unix()
			if sint < info.s {
				sint = info.s
			}
			if eint > info.e {
				eint = info.e
			}
			info.data = append(info.data, []int64{sint, eint})
		}
		if info.num == message.SumNum {
			// 获取到完整数据
			info.resp <- transRecordList(info.data)
		}
		_recordList.Store(recordKey, info)
		return nil
	}
	return errors.New("recordlist devices not found")
}

// Records Records
type Records struct {
	// 存在录像的天数
	DayTotal int          `json:"daynum"`
	TimeNum  int          `json:"timenum"`
	Data     []RecordDate `json:"list"`
}

type RecordDate struct {
	// 日期
	Date string `json:"date"`
	// 时间段
	Items []RecordInfo `json:"items"`
}

type RecordInfo struct {
	Start int64 `json:"start" bson:"start"`
	End   int64 `json:"end" bson:"end"`
}

// 将返回的多组数据合并，时间连续的进行合并，最后按照天返回数据，返回为某天内时间段列表
func transRecordList(data [][]int64) Records {
	if len(data) == 0 {
		return Records{}
	}
	res := Records{}
	list := map[string][]RecordInfo{}
	sort.Slice(data, func(i, j int) bool {
		return data[i][0] < data[j][0]
	})
	newData := [][]int64{}
	newDataIE := []int64{}

	for x, d := range data {
		if x == 0 {
			newDataIE = d
			continue
		}
		if d[0] == newDataIE[1] {
			newDataIE[1] = d[1]
		} else {
			newData = append(newData, newDataIE)
			newDataIE = d
		}
	}
	newData = append(newData, newDataIE)
	var cs, ce time.Time
	dates := []string{}
	for _, d := range newData {
		s := time.Unix(d[0], 0)
		e := time.Unix(d[1], 0)
		cs, _ = time.ParseInLocation("20060102", s.Format("20060102"), time.Local)
		for {
			ce = cs.Add(24 * time.Hour)
			if e.Unix() >= ce.Unix() {
				// 当前时段跨天
				if v, ok := list[cs.Format("2006-01-02")]; ok {
					list[cs.Format("2006-01-02")] = append(v, RecordInfo{
						Start: sip.Max(s.Unix(), cs.Unix()),
						End:   ce.Unix() - 1,
					})
				} else {
					list[cs.Format("2006-01-02")] = []RecordInfo{
						{
							Start: sip.Max(s.Unix(), cs.Unix()),
							End:   ce.Unix() - 1,
						},
					}
					dates = append(dates, cs.Format("2006-01-02"))
					res.DayTotal++
				}
				res.TimeNum++
				cs = ce
			} else {
				if v, ok := list[cs.Format("2006-01-02")]; ok {
					list[cs.Format("2006-01-02")] = append(v, RecordInfo{
						Start: sip.Max(s.Unix(), cs.Unix()),
						End:   e.Unix(),
					})
				} else {
					list[cs.Format("2006-01-02")] = []RecordInfo{
						{
							Start: sip.Max(s.Unix(), cs.Unix()),
							End:   e.Unix(),
						},
					}
					dates = append(dates, cs.Format("2006-01-02"))
					res.DayTotal++
				}
				res.TimeNum++
				break
			}
		}
	}
	resData := []RecordDate{}
	for _, date := range dates {
		resData = append(resData, RecordDate{
			Date:  date,
			Items: list[date],
		})
	}
	res.Data = resData
	return res
}
