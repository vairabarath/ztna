package enrollmentgrpc

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/igris/ztna/controller/internal/device"
	"github.com/igris/ztna/controller/internal/enrollment"
	"github.com/igris/ztna/controller/internal/revocation"
	"github.com/igris/ztna/controller/internal/token"
	"github.com/igris/ztna/controller/internal/workspace"
	controlplanev1 "github.com/igris/ztna/proto/gen/go/ztna/controlplane/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Handler struct {
	controlplanev1.UnimplementedEnrollmentServiceServer

	Svc           enrollment.Service
	WorkspaceSvc  workspace.Service
	DeviceRepo    device.Repository
	RevocationSvc revocation.Service
	RequireMTLS   bool
}

func NewHandler(
	svc enrollment.Service,
	workspaceSvc workspace.Service,
	deviceRepo device.Repository,
	revocationSvc revocation.Service,
	requireMTLS bool,
) *Handler {
	return &Handler{
		Svc:           svc,
		WorkspaceSvc:  workspaceSvc,
		DeviceRepo:    deviceRepo,
		RevocationSvc: revocationSvc,
		RequireMTLS:   requireMTLS,
	}
}

func (h *Handler) Enroll(ctx context.Context, req *controlplanev1.EnrollRequest) (*controlplanev1.EnrollResponse, error) {
	if h.Svc == nil {
		return nil, status.Error(codes.FailedPrecondition, "enrollment service is not configured")
	}
	if req.GetWorkspaceId() == "" || req.GetBootstrapToken() == "" || req.GetCsrPem() == "" || req.GetDeviceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id, bootstrap_token, csr_pem and device_id are required")
	}

	typeValue, err := mapProtoTokenType(req.GetType())
	if err != nil {
		return nil, err
	}

	out, svcErr := h.Svc.Enroll(ctx, enrollment.EnrollInput{
		WorkspaceID:    req.GetWorkspaceId(),
		BootstrapToken: req.GetBootstrapToken(),
		Type:           typeValue,
		CSRPEM:         req.GetCsrPem(),
		DeviceID:       req.GetDeviceId(),
		Hostname:       req.GetHostname(),
		Metadata:       req.GetMetadata(),
	})
	if svcErr != nil {
		return nil, mapServiceErr(svcErr)
	}

	return &controlplanev1.EnrollResponse{
		CertificatePem:  out.CertificatePEM,
		CaCertPem:       out.CACertPEM,
		CertFingerprint: out.CertFingerprint,
		ExpiresAt:       timestamppb.New(out.ExpiresAt),
	}, nil
}

func (h *Handler) Renew(ctx context.Context, req *controlplanev1.RenewRequest) (*controlplanev1.RenewResponse, error) {
	if h.Svc == nil {
		return nil, status.Error(codes.FailedPrecondition, "enrollment service is not configured")
	}
	if h.WorkspaceSvc == nil {
		return nil, status.Error(codes.FailedPrecondition, "workspace service is not configured")
	}
	if h.DeviceRepo == nil {
		return nil, status.Error(codes.FailedPrecondition, "device repository is not configured")
	}
	if req.GetWorkspaceId() == "" || req.GetCsrPem() == "" || req.GetDeviceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id, csr_pem and device_id are required")
	}
	if h.RequireMTLS {
		if err := h.verifyRenewPeer(ctx, req.GetWorkspaceId(), req.GetDeviceId()); err != nil {
			return nil, err
		}
	}

	out, svcErr := h.Svc.Renew(ctx, enrollment.RenewInput{
		WorkspaceID: req.GetWorkspaceId(),
		CSRPEM:      req.GetCsrPem(),
		DeviceID:    req.GetDeviceId(),
	})
	if svcErr != nil {
		return nil, mapServiceErr(svcErr)
	}

	return &controlplanev1.RenewResponse{
		CertificatePem:  out.CertificatePEM,
		CaCertPem:       out.CACertPEM,
		CertFingerprint: out.CertFingerprint,
		ExpiresAt:       timestamppb.New(out.ExpiresAt),
	}, nil
}

func (h *Handler) verifyRenewPeer(ctx context.Context, workspaceID, deviceID string) error {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing peer information")
	}
	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return status.Error(codes.Unauthenticated, "mTLS required for renew")
	}
	if len(tlsInfo.State.PeerCertificates) == 0 {
		return status.Error(codes.Unauthenticated, "client certificate is required")
	}
	clientCert := tlsInfo.State.PeerCertificates[0]

	caCertPEM, err := h.WorkspaceSvc.GetWorkspaceCA(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, workspace.ErrNotFound) {
			return status.Error(codes.NotFound, err.Error())
		}
		return status.Errorf(codes.Internal, "load workspace CA: %v", err)
	}
	caCert, err := parseCertPEM(caCertPEM)
	if err != nil {
		return status.Errorf(codes.Internal, "parse workspace CA: %v", err)
	}

	roots := x509.NewCertPool()
	roots.AddCert(caCert)
	if _, err := clientCert.Verify(x509.VerifyOptions{
		Roots:       roots,
		CurrentTime: time.Now().UTC(),
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err != nil {
		return status.Error(codes.PermissionDenied, "client certificate is not trusted by workspace CA")
	}

	expectedURI := fmt.Sprintf("ztna://%s/%s", workspaceID, deviceID)
	expectedDNS := fmt.Sprintf("%s.%s", deviceID, workspaceID)
	if !hasSANURI(clientCert, expectedURI) || !hasSANDNS(clientCert, expectedDNS) {
		return status.Error(codes.PermissionDenied, "client certificate workspace/device SAN mismatch")
	}

	fingerprint := certFingerprint(clientCert)
	deviceRecord, err := h.DeviceRepo.GetByID(ctx, workspaceID, deviceID)
	if err != nil {
		if errors.Is(err, device.ErrNotFound) {
			return status.Error(codes.PermissionDenied, "device is not registered")
		}
		return status.Errorf(codes.Internal, "load device record: %v", err)
	}
	if deviceRecord.Status != "active" {
		return status.Error(codes.PermissionDenied, "device is not active")
	}
	if deviceRecord.CertFingerprint != fingerprint {
		return status.Error(codes.PermissionDenied, "client certificate fingerprint does not match active device record")
	}

	if h.RevocationSvc != nil {
		revoked, err := h.RevocationSvc.IsRevoked(ctx, workspaceID, fingerprint)
		if err != nil {
			return status.Errorf(codes.Internal, "check revocation status: %v", err)
		}
		if revoked {
			return status.Error(codes.PermissionDenied, "client certificate is revoked")
		}
	}

	return nil
}

func parseCertPEM(certPEM string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, fmt.Errorf("invalid cert pem")
	}
	return x509.ParseCertificate(block.Bytes)
}

func hasSANURI(cert *x509.Certificate, want string) bool {
	for _, u := range cert.URIs {
		if u.String() == want {
			return true
		}
	}
	return false
}

func hasSANDNS(cert *x509.Certificate, want string) bool {
	for _, dns := range cert.DNSNames {
		if dns == want {
			return true
		}
	}
	return false
}

func certFingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:])
}

func mapServiceErr(err error) error {
	switch {
	case errors.Is(err, enrollment.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, enrollment.ErrWorkspaceNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, enrollment.ErrCSRInvalid):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, enrollment.ErrCSRSignatureInvalid):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, enrollment.ErrCSRWorkspaceMismatch):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, enrollment.ErrCSRDeviceMismatch):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, enrollment.ErrCSRMissingSAN):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, token.ErrTokenNotFound):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, token.ErrTokenExpired):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, token.ErrTokenUsed):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, token.ErrTokenType):
		return status.Error(codes.PermissionDenied, err.Error())
	default:
		return status.Errorf(codes.Internal, "internal error: %v", err)
	}
}

func mapProtoTokenType(in controlplanev1.EnrollTokenType) (string, error) {
	switch in {
	case controlplanev1.EnrollTokenType_ENROLL_TOKEN_TYPE_CONNECTOR:
		return token.TypeConnector, nil
	case controlplanev1.EnrollTokenType_ENROLL_TOKEN_TYPE_AGENT:
		return token.TypeAgent, nil
	default:
		return "", status.Error(codes.InvalidArgument, "token type must be CONNECTOR or AGENT")
	}
}
