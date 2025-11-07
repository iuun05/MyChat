package rpc

import (
	"MyChat/models"
	pb "MyChat/protobuf"
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// MessageServiceClient gRPC 客户端封装
type MessageServiceClient struct {
	conn   *grpc.ClientConn
	client pb.MessageServiceClient
	ctx    context.Context
	cancel context.CancelFunc
}

// NewMessageServiceClient 创建新的 gRPC 客户端
func NewMessageServiceClient(addr string) (*MessageServiceClient, error) {
	// 建立连接
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()), // 生产环境应使用 TLS
	)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	client := &MessageServiceClient{
		conn:   conn,
		client: pb.NewMessageServiceClient(conn),
		ctx:    ctx,
		cancel: cancel,
	}

	zap.S().Infof("gRPC客户端连接成功: %s", addr)
	return client, nil
}

// Close 关闭客户端连接
func (c *MessageServiceClient) Close() error {
	c.cancel()
	return c.conn.Close()
}

// SendMessage 发送单条消息
func (c *MessageServiceClient) SendMessage(msg *models.Message) (*pb.MessageResponse, error) {
	// 转换 models.Message 到 pb.MessageRequest
	req := c.modelToPbMessage(msg)

	// 调用 gRPC
	resp, err := c.client.SendMessage(c.ctx, req)
	if err != nil {
		zap.S().Error("发送消息失败", zap.Error(err))
		return nil, err
	}

	zap.S().Debugf("消息发送成功 seq=%d, success=%v", resp.Seq, resp.Success)
	return resp, nil
}

// StreamMessages 流式发送消息（支持批量）
// 返回一个通道用于发送消息，一个通道用于接收响应
func (c *MessageServiceClient) StreamMessages() (chan *models.Message, chan *pb.MessageResponse, error) {
	// 创建流
	stream, err := c.client.StreamMessages(c.ctx)
	if err != nil {
		return nil, nil, err
	}

	// 创建通道
	sendChan := make(chan *models.Message, 100)
	recvChan := make(chan *pb.MessageResponse, 100)

	// 发送协程
	go func() {
		defer close(sendChan)
		for msg := range sendChan {
			req := c.modelToPbMessage(msg)
			if err := stream.Send(req); err != nil {
				zap.S().Error("流式发送消息失败", zap.Error(err))
				return
			}
		}
		stream.CloseSend()
	}()

	// 接收协程
	go func() {
		defer close(recvChan)
		for {
			resp, err := stream.Recv()
			if err != nil {
				if err.Error() != "EOF" {
					zap.S().Error("流式接收消息失败", zap.Error(err))
				}
				return
			}
			recvChan <- resp
		}
	}()

	return sendChan, recvChan, nil
}

// Heartbeat 发送心跳
func (c *MessageServiceClient) Heartbeat(userId int64) (*pb.HeartbeatResponse, error) {
	req := &pb.HeartbeatRequest{
		UserId:    userId,
		Timestamp: timestamppb.Now(),
	}

	resp, err := c.client.Heartbeat(c.ctx, req)
	if err != nil {
		zap.S().Error("发送心跳失败", zap.Error(err))
		return nil, err
	}

	zap.S().Debugf("心跳成功 userId=%d, alive=%v", userId, resp.Alive)
	return resp, nil
}

// AcknowledgeMessage 发送消息确认
func (c *MessageServiceClient) AcknowledgeMessage(seq, fromId, targetId int64) (*pb.AckResponse, error) {
	req := &pb.AckRequest{
		Seq:      seq,
		FromId:   fromId,
		TargetId: targetId,
	}

	resp, err := c.client.AcknowledgeMessage(c.ctx, req)
	if err != nil {
		zap.S().Error("发送ACK失败", zap.Error(err))
		return nil, err
	}

	zap.S().Debugf("ACK发送成功 seq=%d, success=%v", seq, resp.Success)
	return resp, nil
}

// StartHeartbeat 启动心跳循环
// interval: 心跳间隔（秒）
func (c *MessageServiceClient) StartHeartbeat(userId int64, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_, err := c.Heartbeat(userId)
			if err != nil {
				zap.S().Warn("心跳失败，可能连接已断开", zap.Error(err))
				return
			}
		case <-c.ctx.Done():
			return
		}
	}
}

// modelToPbMessage 将 models.Message 转换为 pb.MessageRequest
func (c *MessageServiceClient) modelToPbMessage(msg *models.Message) *pb.MessageRequest {
	req := &pb.MessageRequest{
		FromId:   msg.FromId,
		TargetId: msg.TargetId,
		Content:  msg.Content,
		Pic:      msg.Pic,
		Url:      msg.Url,
		Desc:     msg.Desc,
		Amount:   int32(msg.Amount),
	}

	// 设置序号
	if msg.Seq > 0 {
		req.Seq = msg.Seq
	}

	// 转换消息类型
	switch msg.Type {
	case models.SingleMessageType:
		req.Type = pb.MessageType_MESSAGE_TYPE_SINGLE
	case models.CommunityMessageType:
		req.Type = pb.MessageType_MESSAGE_TYPE_COMMUNITY
	case models.BroadcastMessageType:
		req.Type = pb.MessageType_MESSAGE_TYPE_BROADCAST
	case models.HeartBeatMessageType:
		req.Type = pb.MessageType_MESSAGE_TYPE_HEARTBEAT
	case models.AckMessageType:
		req.Type = pb.MessageType_MESSAGE_TYPE_ACK
	default:
		req.Type = pb.MessageType_MESSAGE_TYPE_SINGLE
	}

	// 转换媒体类型
	switch msg.Media {
	case 1: // 文字
		req.Media = pb.MediaType_MEDIA_TYPE_TEXT
	case 2: // 图片
		req.Media = pb.MediaType_MEDIA_TYPE_IMAGE
	case 3: // 音频
		req.Media = pb.MediaType_MEDIA_TYPE_AUDIO
	case 4: // 视频
		req.Media = pb.MediaType_MEDIA_TYPE_VIDEO
	case 5: // 文件
		req.Media = pb.MediaType_MEDIA_TYPE_FILE
	default:
		req.Media = pb.MediaType_MEDIA_TYPE_TEXT
	}

	// 设置时间戳
	if !msg.CreatedAt.IsZero() {
		req.Timestamp = timestamppb.New(msg.CreatedAt)
	} else {
		req.Timestamp = timestamppb.Now()
	}

	return req
}
