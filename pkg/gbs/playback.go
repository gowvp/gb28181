package gbs

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/gowvp/gb28181/internal/core/ipc"
	"github.com/gowvp/gb28181/internal/core/sms"
	"github.com/gowvp/gb28181/pkg/gbs/sip"
	"github.com/gowvp/gb28181/pkg/zlm"
	sdp "github.com/panjjo/gosdp"
)

// PlaybackInput 录像回放输入
type PlaybackInput struct {
	Channel    *ipc.Channel
	SMS        *sms.MediaServer
	StreamMode int8
	StartTime  int64 // 开始时间戳(秒)
	EndTime    int64 // 结束时间戳(秒)
}

// PlaybackControl 回放控制类型
type PlaybackControl string

const (
	PlaybackControlPlay     PlaybackControl = "PLAY"     // 播放
	PlaybackControlPause    PlaybackControl = "PAUSE"    // 暂停
	PlaybackControlTeardown PlaybackControl = "TEARDOWN" // 停止
	PlaybackControlScale    PlaybackControl = "SCALE"    // 倍速
)

// StopPlaybackInput 停止回放输入
type StopPlaybackInput struct {
	Channel *ipc.Channel
}

// Playback 录像回放
func (g *GB28181API) Playback(in *PlaybackInput) error {
	log := slog.With("deviceID", in.Channel.DeviceID, "channelID", in.Channel.ChannelID,
		"startTime", in.StartTime, "endTime", in.EndTime)
	log.Info("开始回放流程")

	ch, ok := g.svr.memoryStorer.GetChannel(in.Channel.DeviceID, in.Channel.ChannelID)
	if !ok {
		log.Error("通道不存在")
		return ErrChannelNotExist
	}

	ch.device.playMutex.Lock()
	defer ch.device.playMutex.Unlock()

	if !ch.device.IsOnline {
		return ErrDeviceOffline
	}

	// 生成回放流 ID
	streamID := fmt.Sprintf("playback_%s_%d", in.Channel.ID, in.StartTime)
	key := "playback:" + in.Channel.DeviceID + ":" + in.Channel.ChannelID

	stream, ok := g.streams.LoadOrStore(key, &Streams{})
	if ok {
		log.Debug("PLAYBACK 已存在流，先停止")
		if err := g.stopPlayback(ch, &StopPlaybackInput{Channel: in.Channel}); err != nil {
			slog.Error("stop playback failed", "err", err)
		}
	}

	log.Debug("1. 开启RTP服务器等待接收视频流")
	resp, err := g.sms.OpenRTPServer(in.SMS, zlm.OpenRTPServerRequest{
		TCPMode:  in.StreamMode,
		StreamID: streamID,
	})
	if err != nil {
		log.Debug("1.1. 开启RTP服务器失败", "err", err)
		return err
	}

	log.Debug("2. 发送回放SDP请求", "port", resp.Port)
	if err := g.sipPlaybackPush(ch, in, resp.Port, stream, streamID); err != nil {
		log.Debug("2.1. 发送回放SDP请求失败", "err", err)
		return err
	}

	return nil
}

// sipPlaybackPush 发送回放请求
func (g *GB28181API) sipPlaybackPush(ch *Channel, in *PlaybackInput, port int, stream *Streams, streamID string) error {
	protocal := "TCP/RTP/AVP"
	if in.StreamMode == 0 {
		protocal = "RTP/AVP"
	}

	video := sdp.Media{
		Description: sdp.MediaDescription{
			Type:     "video",
			Port:     port,
			Formats:  []string{"96", "97", "98"},
			Protocol: protocal,
		},
	}
	video.AddAttribute("recvonly")

	switch in.StreamMode {
	case 1:
		video.AddAttribute("setup", "passive")
		video.AddAttribute("connection", "new")
	case 2:
		video.AddAttribute("setup", "active")
		video.AddAttribute("connection", "new")
	}
	video.AddAttribute("rtpmap", "96", "PS/90000")
	video.AddAttribute("rtpmap", "97", "MPEG4/90000")
	video.AddAttribute("rtpmap", "98", "H264/90000")

	ipstr := in.SMS.GetSDPIP()
	ip4str, err := GetIP(ipstr)
	if err != nil {
		slog.Error("域名解析失败", "域名", ipstr, "错误", err)
		return err
	}

	// 回放需要设置时间范围
	msg := &sdp.Message{
		Origin: sdp.Origin{
			Username:    ch.ChannelID,
			NetworkType: "IN",
			AddressType: "IP4",
			Address:     ip4str,
		},
		Name: "Playback", // 回放使用 Playback
		Connection: sdp.ConnectionData{
			NetworkType: "IN",
			AddressType: "IP4",
			IP:          net.ParseIP(ip4str),
		},
		Timing: []sdp.Timing{
			{
				Start: time.Unix(in.StartTime, 0),
				End:   time.Unix(in.EndTime, 0),
			},
		},
		Medias: []sdp.Media{video},
		SSRC:   g.getSSRC(1),                          // 回放类型
		URI:    fmt.Sprintf("%s:0", ch.ChannelID),     // 回放需要 URI
	}

	body := msg.Append(nil).AppendTo(nil)

	slog.Info("回放SDP>>>", "body", string(body))

	tx, err := g.svr.wrapRequest(ch, sip.MethodInvite, &sip.ContentTypeSDP, body, func(r *sip.Request) {
		r.AppendHeader(&sip.GenericHeader{HeaderName: "Subject", Contents: fmt.Sprintf("%s:%s,%s:%s", ch.ChannelID, streamID, in.Channel.DeviceID, streamID)})
	})
	if err != nil {
		return err
	}
	resp, err := sipResponse(tx)
	if err != nil {
		return err
	}

	if contact, _ := resp.Contact(); contact == nil {
		resp.AppendHeader(&sip.ContactHeader{
			DisplayName: g.svr.fromAddress.DisplayName,
			Address:     &sip.URI{FUser: sip.String{Str: g.cfg.ID}, FHost: g.cfg.Domain},
			Params:      sip.NewParams(),
		})
	}

	stream.Resp = resp

	ackReq := sip.NewRequestFromResponse(sip.MethodACK, resp)
	return tx.Request(ackReq)
}

// stopPlayback 停止回放
func (g *GB28181API) stopPlayback(ch *Channel, in *StopPlaybackInput) error {
	key := "playback:" + in.Channel.DeviceID + ":" + in.Channel.ChannelID
	stream, ok := g.streams.LoadAndDelete(key)
	if !ok {
		return nil
	}

	if stream.Resp == nil {
		return nil
	}

	req := sip.NewRequestFromResponse(sip.MethodBYE, stream.Resp)
	req.SetDestination(ch.Source())
	req.SetConnection(ch.Conn())

	_, err := g.svr.Request(req)
	return err
}

// StopPlayback 停止回放 (加锁)
func (g *GB28181API) StopPlayback(ctx context.Context, in *StopPlaybackInput) error {
	ch, ok := g.svr.memoryStorer.GetChannel(in.Channel.DeviceID, in.Channel.ChannelID)
	if !ok {
		return ErrChannelNotExist
	}

	ch.device.playMutex.Lock()
	defer ch.device.playMutex.Unlock()

	return g.stopPlayback(ch, in)
}

// PlaybackControl 回放控制 (暂停/继续/快进等)
func (g *GB28181API) PlaybackControl(deviceID, channelID string, control PlaybackControl, scale float64) error {
	ch, ok := g.svr.memoryStorer.GetChannel(deviceID, channelID)
	if !ok {
		return ErrChannelNotExist
	}

	key := "playback:" + deviceID + ":" + channelID
	stream, ok := g.streams.Load(key)
	if !ok || stream.Resp == nil {
		return fmt.Errorf("playback session not found")
	}

	var body string
	switch control {
	case PlaybackControlPause:
		body = "PAUSE RTSP/1.0\r\nCSeq: 1\r\n\r\n"
	case PlaybackControlPlay:
		body = "PLAY RTSP/1.0\r\nCSeq: 2\r\n\r\n"
	case PlaybackControlScale:
		body = fmt.Sprintf("PLAY RTSP/1.0\r\nCSeq: 3\r\nScale: %.1f\r\n\r\n", scale)
	default:
		return fmt.Errorf("unsupported control type: %s", control)
	}

	req := sip.NewRequestFromResponse(sip.MethodInfo, stream.Resp)
	req.SetDestination(ch.Source())
	req.SetBody([]byte(body), true)

	_, err := g.svr.Request(req)
	return err
}

// Server 方法

// Playback 录像回放
func (s *Server) Playback(in *PlaybackInput) error {
	return s.gb.Playback(in)
}

// StopPlayback 停止回放
func (s *Server) StopPlayback(ctx context.Context, in *StopPlaybackInput) error {
	return s.gb.StopPlayback(ctx, in)
}

// PlaybackControl 回放控制
func (s *Server) PlaybackControl(deviceID, channelID string, control PlaybackControl, scale float64) error {
	return s.gb.PlaybackControl(deviceID, channelID, control, scale)
}

// QueryRecordInfo 查询录像信息
func (s *Server) QueryRecordInfo(deviceID, channelID string, startTime, endTime int64) (*Records, error) {
	ch := &Channels{
		DeviceID:  deviceID,
		ChannelID: channelID,
	}
	return SipRecordList(ch, startTime, endTime)
}
