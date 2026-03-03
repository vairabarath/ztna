package devicegrpc

import (
	"context"
	"errors"
	"time"

	"github.com/igris/ztna/controller/internal/device"
	"github.com/igris/ztna/controller/internal/revocation"
	controlplanev1 "github.com/igris/ztna/proto/gen/go/ztna/controlplane/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Handler struct {
	controlplanev1.UnimplementedDeviceServiceServer

	DeviceRepo    device.Repository
	RevocationSvc revocation.Service
}

func NewHandler(deviceRepo device.Repository, revocationSvc revocation.Service) *Handler {
	return &Handler{DeviceRepo: deviceRepo, RevocationSvc: revocationSvc}
}

func (h *Handler) StreamRevocations(
	req *controlplanev1.StreamRevocationsRequest,
	stream controlplanev1.DeviceService_StreamRevocationsServer,
) error {
	if h.RevocationSvc == nil {
		return status.Error(codes.FailedPrecondition, "revocation service is not configured")
	}
	if req.GetWorkspaceId() == "" {
		return status.Error(codes.InvalidArgument, "workspace_id is required")
	}

	ch, cancel := h.RevocationSvc.Subscribe(req.GetWorkspaceId())
	defer cancel()

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case ev := <-ch:
			if err := stream.Send(&controlplanev1.RevocationEvent{
				WorkspaceId:     ev.WorkspaceID,
				DeviceId:        ev.DeviceID,
				CertFingerprint: ev.CertFingerprint,
				Reason:          ev.Reason,
				RevokedAt:       timestamppb.New(time.UnixMilli(ev.RevokedUnixMilli).UTC()),
			}); err != nil {
				return status.Errorf(codes.Unavailable, "send revocation event: %v", err)
			}
		}
	}
}

func (h *Handler) Heartbeat(ctx context.Context, req *controlplanev1.HeartbeatRequest) (*controlplanev1.HeartbeatResponse, error) {
	if req.GetWorkspaceId() == "" || req.GetDeviceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id and device_id are required")
	}

	if h.DeviceRepo != nil {
		_ = h.DeviceRepo.Upsert(ctx, device.Device{
			WorkspaceID:      req.GetWorkspaceId(),
			DeviceID:         req.GetDeviceId(),
			CertFingerprint:  req.GetCertFingerprint(),
			Status:           "active",
			LastSeenUnixTime: time.Now().UTC().Unix(),
		})
	}

	return &controlplanev1.HeartbeatResponse{ServerTime: timestamppb.Now()}, nil
}

func (h *Handler) RevokeDevice(ctx context.Context, req *controlplanev1.RevokeDeviceRequest) (*controlplanev1.RevokeDeviceResponse, error) {
	if h.DeviceRepo == nil {
		return nil, status.Error(codes.FailedPrecondition, "device repository is not configured")
	}
	if h.RevocationSvc == nil {
		return nil, status.Error(codes.FailedPrecondition, "revocation service is not configured")
	}
	if req.GetWorkspaceId() == "" || req.GetDeviceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id and device_id are required")
	}

	dev, err := h.DeviceRepo.GetByID(ctx, req.GetWorkspaceId(), req.GetDeviceId())
	if err != nil {
		if errors.Is(err, device.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "device not found")
		}
		return nil, status.Errorf(codes.Internal, "get device: %v", err)
	}
	if dev.Status != "active" {
		return nil, status.Error(codes.FailedPrecondition, "device is not active")
	}

	reason := req.GetReason()
	if reason == "" {
		reason = "manual revocation"
	}
	revokedAt := time.Now().UTC()
	revokeErr := h.RevocationSvc.Revoke(ctx, revocation.Entry{
		WorkspaceID:      req.GetWorkspaceId(),
		DeviceID:         req.GetDeviceId(),
		CertFingerprint:  dev.CertFingerprint,
		Reason:           reason,
		RevokedUnixMilli: revokedAt.UnixMilli(),
	})
	if revokeErr != nil {
		if errors.Is(revokeErr, revocation.ErrAlreadyRevoked) {
			return nil, status.Error(codes.FailedPrecondition, revokeErr.Error())
		}
		return nil, status.Errorf(codes.Internal, "revoke device certificate: %v", revokeErr)
	}

	if err := h.DeviceRepo.UpdateStatus(ctx, req.GetWorkspaceId(), req.GetDeviceId(), "revoked"); err != nil {
		if errors.Is(err, device.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "device not found")
		}
		return nil, status.Errorf(codes.Internal, "update device status: %v", err)
	}

	return &controlplanev1.RevokeDeviceResponse{
		WorkspaceId:     req.GetWorkspaceId(),
		DeviceId:        req.GetDeviceId(),
		CertFingerprint: dev.CertFingerprint,
		RevokedAt:       timestamppb.New(revokedAt),
	}, nil
}

func (h *Handler) ListDevices(ctx context.Context, req *controlplanev1.ListDevicesRequest) (*controlplanev1.ListDevicesResponse, error) {
	if h.DeviceRepo == nil {
		return nil, status.Error(codes.FailedPrecondition, "device repository is not configured")
	}
	if req.GetWorkspaceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id is required")
	}

	devices, err := h.DeviceRepo.ListByWorkspace(ctx, req.GetWorkspaceId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list devices: %v", err)
	}

	items := make([]*controlplanev1.DeviceInfo, 0, len(devices))
	for _, d := range devices {
		items = append(items, &controlplanev1.DeviceInfo{
			WorkspaceId:     d.WorkspaceID,
			DeviceId:        d.DeviceID,
			CertFingerprint: d.CertFingerprint,
			Status:          d.Status,
			LastSeenAt:      timestamppb.New(time.Unix(d.LastSeenUnixTime, 0).UTC()),
		})
	}

	return &controlplanev1.ListDevicesResponse{Items: items}, nil
}
