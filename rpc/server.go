package rpc

import (
	"MyChat/dao"
	"MyChat/models"
	pb "MyChat/protobuf"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// MessageServiceImpl 实现 MessageService 接口
type MessageServiceImpl struct {
	pb.UnimplementedMessageServiceServer
	messageDAO *dao.MessageDAO
}

// NewMessageServiceImpl 创建MessageService实现
func NewMessageServiceImpl() *MessageServiceImpl {
	return &MessageServiceImpl{
		messageDAO: dao.NewMessageDAO(),
	}
}

// SendMessage 发送消息（单播）
func (s *MessageServiceImpl) SendMessage(ctx context.Context, req *pb.MessageRequest) (*pb.MessageResponse, error) {
	// 转换protobuf消息到models.Message
	msg, err := s.pbToModelMessage(req)
	if err != nil {
		zap.S().Error("转换消息失败", zap.Error(err))
		return &pb.MessageResponse{
			Seq:      0,
			Success:  false,
			ErrorMsg: "消息格式错误: " + err.Error(),
		}, status.Error(codes.InvalidArgument, err.Error())
	}

	// 序列化为JSON
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		zap.S().Error("序列化消息失败", zap.Error(err))
		return &pb.MessageResponse{
			Seq:      0,
			Success:  false,
			ErrorMsg: "消息序列化失败: " + err.Error(),
		}, status.Error(codes.Internal, err.Error())
	}

	// 调用dao层处理消息
	dao.Dispatch(msgBytes)

	return &pb.MessageResponse{
		Seq:      msg.Seq,
		Success:  true,
		ErrorMsg: "",
	}, nil
}

// StreamMessages 流式发送消息（支持批量）
func (s *MessageServiceImpl) StreamMessages(stream grpc.BidiStreamingServer[pb.MessageRequest, pb.MessageResponse]) error {
	// 启动一个协程监听 context 取消
	ctxDone := make(chan struct{})
	go func() {
		<-stream.Context().Done()
		close(ctxDone)
	}()

	for {
		// 接收消息
		req, err := stream.Recv()
		if err != nil {
			// 流结束
			if err == io.EOF {
				return nil
			}
			zap.S().Error("接收流消息失败", zap.Error(err))
			return err
		}

		// 处理消息
		msg, err := s.pbToModelMessage(req)
		if err != nil {
			zap.S().Error("转换流消息失败", zap.Error(err))
			// 发送错误响应
			stream.Send(&pb.MessageResponse{
				Seq:      0,
				Success:  false,
				ErrorMsg: "消息格式错误: " + err.Error(),
			})
			continue
		}

		// 序列化为JSON
		msgBytes, err := json.Marshal(msg)
		if err != nil {
			zap.S().Error("序列化流消息失败", zap.Error(err))
			stream.Send(&pb.MessageResponse{
				Seq:      0,
				Success:  false,
				ErrorMsg: "消息序列化失败: " + err.Error(),
			})
			continue
		}

		// 调用dao层处理消息
		dao.Dispatch(msgBytes)

		// 发送成功响应
		if err := stream.Send(&pb.MessageResponse{
			Seq:      msg.Seq,
			Success:  true,
			ErrorMsg: "",
		}); err != nil {
			zap.S().Error("发送流响应失败", zap.Error(err))
			return err
		}

		// 检查context是否被取消
		select {
		case <-ctxDone:
			return stream.Context().Err()
		default:
		}
	}
}

// Heartbeat 心跳检测
func (s *MessageServiceImpl) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	if req.UserId == 0 {
		return &pb.HeartbeatResponse{
			Alive: false,
		}, status.Error(codes.InvalidArgument, "user_id is required")
	}

	zap.S().Debugf("收到心跳请求 userId=%d", req.UserId)

	return &pb.HeartbeatResponse{
		Alive:     true,
		Timestamp: timestamppb.Now(),
	}, nil
}

// AcknowledgeMessage 消息确认
func (s *MessageServiceImpl) AcknowledgeMessage(ctx context.Context, req *pb.AckRequest) (*pb.AckResponse, error) {
	if req.Seq == 0 {
		return &pb.AckResponse{
			Success: false,
		}, status.Error(codes.InvalidArgument, "seq is required")
	}

	if req.FromId == 0 || req.TargetId == 0 {
		return &pb.AckResponse{
			Success: false,
		}, status.Error(codes.InvalidArgument, "from_id and target_id are required")
	}

	// 创建ACK消息
	ackMsg := models.Message{
		Seq:      req.Seq,
		FromId:   req.FromId,
		TargetId: req.TargetId,
		Type:     models.AckMessageType,
		Content:  "ack",
	}

	// 序列化为JSON
	ackBytes, err := json.Marshal(ackMsg)
	if err != nil {
		zap.S().Error("序列化ACK消息失败", zap.Error(err))
		return &pb.AckResponse{
			Success: false,
		}, status.Error(codes.Internal, err.Error())
	}

	// 调用dao层处理ACK
	dao.Dispatch(ackBytes)

	zap.S().Debugf("处理ACK确认 seq=%d, fromId=%d, targetId=%d", req.Seq, req.FromId, req.TargetId)

	return &pb.AckResponse{
		Success: true,
		AckSeq:  req.Seq,
	}, nil
}

// pbToModelMessage 将protobuf消息转换为models.Message
func (s *MessageServiceImpl) pbToModelMessage(req *pb.MessageRequest) (*models.Message, error) {
	// 验证必填字段
	if req.FromId == 0 {
		return nil, fmt.Errorf("from_id is required")
	}
	if req.TargetId == 0 {
		return nil, fmt.Errorf("target_id is required")
	}

	msg := &models.Message{
		FromId:   req.FromId,
		TargetId: req.TargetId,
		Content:  req.Content,
		Pic:      req.Pic,
		Url:      req.Url,
		Desc:     req.Desc,
		Amount:   int(req.Amount),
	}

	// 设置序号（如果客户端未提供，服务器会在 Dispatch 中自动生成）
	if req.Seq > 0 {
		msg.Seq = req.Seq
	}
	// 注意：如果 seq == 0，消息会在 dao.Dispatch -> sendMsgAndSave 中自动分配序号

	// 转换消息类型
	switch req.Type {
	case pb.MessageType_MESSAGE_TYPE_SINGLE:
		msg.Type = models.SingleMessageType
	case pb.MessageType_MESSAGE_TYPE_COMMUNITY:
		msg.Type = models.CommunityMessageType
	case pb.MessageType_MESSAGE_TYPE_BROADCAST:
		msg.Type = models.BroadcastMessageType
	case pb.MessageType_MESSAGE_TYPE_HEARTBEAT:
		msg.Type = models.HeartBeatMessageType
	case pb.MessageType_MESSAGE_TYPE_ACK:
		msg.Type = models.AckMessageType
	default:
		msg.Type = models.SingleMessageType
	}

	// 转换媒体类型
	switch req.Media {
	case pb.MediaType_MEDIA_TYPE_TEXT:
		msg.Media = 1 // 文字
	case pb.MediaType_MEDIA_TYPE_IMAGE:
		msg.Media = 2 // 图片
	case pb.MediaType_MEDIA_TYPE_AUDIO:
		msg.Media = 3 // 音频
	case pb.MediaType_MEDIA_TYPE_VIDEO:
		msg.Media = 4 // 视频
	case pb.MediaType_MEDIA_TYPE_FILE:
		msg.Media = 5 // 文件
	default:
		msg.Media = 1 // 默认文字
	}

	// 设置时间戳
	if req.Timestamp != nil {
		msg.CreatedAt = req.Timestamp.AsTime()
	} else {
		msg.CreatedAt = time.Now()
	}

	// 如果没有设置更新时间，使用创建时间
	if msg.UpdatedAt.IsZero() {
		msg.UpdatedAt = msg.CreatedAt
	}

	return msg, nil
}

// RegisterService 注册gRPC服务
func RegisterService(s *grpc.Server) {
	pb.RegisterMessageServiceServer(s, NewMessageServiceImpl())
}
