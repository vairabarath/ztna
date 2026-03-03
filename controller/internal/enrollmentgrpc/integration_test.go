package enrollmentgrpc_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"net/url"
	"testing"
	"time"

	"github.com/igris/ztna/controller/internal/device"
	"github.com/igris/ztna/controller/internal/devicegrpc"
	"github.com/igris/ztna/controller/internal/enrollment"
	"github.com/igris/ztna/controller/internal/enrollmentgrpc"
	"github.com/igris/ztna/controller/internal/revocation"
	"github.com/igris/ztna/controller/internal/token"
	"github.com/igris/ztna/controller/internal/workspace"
	"github.com/igris/ztna/controller/internal/workspacegrpc"
	controlplanev1 "github.com/igris/ztna/proto/gen/go/ztna/controlplane/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestIntegrationWorkspaceTokenEnrollAndMTLSRenew(t *testing.T) {
	stack := newIntegrationStack(t)

	ws := stack.createWorkspace(t)
	tokenOut := stack.createToken(t, ws.WorkspaceId, controlplanev1.EnrollTokenType_ENROLL_TOKEN_TYPE_CONNECTOR)
	deviceID := "dev-int-1"
	enrolled := stack.enroll(t, ws.WorkspaceId, tokenOut.Token, controlplanev1.EnrollTokenType_ENROLL_TOKEN_TYPE_CONNECTOR, deviceID)

	renewReq := &controlplanev1.RenewRequest{
		WorkspaceId: ws.WorkspaceId,
		DeviceId:    deviceID,
		CsrPem:      mustBuildCSR(t, ws.WorkspaceId, deviceID),
	}
	renewed, err := stack.enrollment.Renew(contextWithMTLSCert(t, enrolled.CertificatePem), renewReq)
	if err != nil {
		t.Fatalf("renew failed: %v", err)
	}
	if renewed.GetCertFingerprint() == "" {
		t.Fatalf("renewed fingerprint should not be empty")
	}
	if renewed.GetCertFingerprint() == enrolled.GetCertFingerprint() {
		t.Fatalf("expected fingerprint rotation on renew")
	}
}

func TestIntegrationTokenReplayRejected(t *testing.T) {
	stack := newIntegrationStack(t)

	ws := stack.createWorkspace(t)
	tokenOut := stack.createToken(t, ws.WorkspaceId, controlplanev1.EnrollTokenType_ENROLL_TOKEN_TYPE_CONNECTOR)
	deviceID := "dev-replay-1"
	_ = stack.enroll(t, ws.WorkspaceId, tokenOut.Token, controlplanev1.EnrollTokenType_ENROLL_TOKEN_TYPE_CONNECTOR, deviceID)

	_, err := stack.enrollment.Enroll(context.Background(), &controlplanev1.EnrollRequest{
		WorkspaceId:    ws.WorkspaceId,
		BootstrapToken: tokenOut.Token,
		Type:           controlplanev1.EnrollTokenType_ENROLL_TOKEN_TYPE_CONNECTOR,
		CsrPem:         mustBuildCSR(t, ws.WorkspaceId, "dev-replay-2"),
		DeviceId:       "dev-replay-2",
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition for token replay, got err=%v code=%v", err, status.Code(err))
	}
}

func TestIntegrationCrossWorkspaceCertRejected(t *testing.T) {
	stack := newIntegrationStack(t)

	wsA := stack.createWorkspace(t)
	wsB := stack.createWorkspace(t)

	tokenA := stack.createToken(t, wsA.WorkspaceId, controlplanev1.EnrollTokenType_ENROLL_TOKEN_TYPE_CONNECTOR)
	deviceID := "dev-cross-1"
	enrolledA := stack.enroll(t, wsA.WorkspaceId, tokenA.Token, controlplanev1.EnrollTokenType_ENROLL_TOKEN_TYPE_CONNECTOR, deviceID)

	_, err := stack.enrollment.Renew(contextWithMTLSCert(t, enrolledA.CertificatePem), &controlplanev1.RenewRequest{
		WorkspaceId: wsB.WorkspaceId,
		DeviceId:    deviceID,
		CsrPem:      mustBuildCSR(t, wsB.WorkspaceId, deviceID),
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied for cross-workspace cert, got err=%v code=%v", err, status.Code(err))
	}
}

func TestIntegrationRevokedCertRejected(t *testing.T) {
	stack := newIntegrationStack(t)

	ws := stack.createWorkspace(t)
	tokenOut := stack.createToken(t, ws.WorkspaceId, controlplanev1.EnrollTokenType_ENROLL_TOKEN_TYPE_CONNECTOR)
	deviceID := "dev-revoked-1"
	enrolled := stack.enroll(t, ws.WorkspaceId, tokenOut.Token, controlplanev1.EnrollTokenType_ENROLL_TOKEN_TYPE_CONNECTOR, deviceID)

	_, err := stack.devices.RevokeDevice(context.Background(), &controlplanev1.RevokeDeviceRequest{
		WorkspaceId: ws.WorkspaceId,
		DeviceId:    deviceID,
		Reason:      "integration-test",
	})
	if err != nil {
		t.Fatalf("revoke device failed: %v", err)
	}

	_, err = stack.enrollment.Renew(contextWithMTLSCert(t, enrolled.CertificatePem), &controlplanev1.RenewRequest{
		WorkspaceId: ws.WorkspaceId,
		DeviceId:    deviceID,
		CsrPem:      mustBuildCSR(t, ws.WorkspaceId, deviceID),
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied for revoked cert, got err=%v code=%v", err, status.Code(err))
	}
}

func TestIntegrationRenewalInvalidatesOldFingerprint(t *testing.T) {
	stack := newIntegrationStack(t)

	ws := stack.createWorkspace(t)
	tokenOut := stack.createToken(t, ws.WorkspaceId, controlplanev1.EnrollTokenType_ENROLL_TOKEN_TYPE_CONNECTOR)
	deviceID := "dev-rotate-1"
	enrolled := stack.enroll(t, ws.WorkspaceId, tokenOut.Token, controlplanev1.EnrollTokenType_ENROLL_TOKEN_TYPE_CONNECTOR, deviceID)

	renewReq := &controlplanev1.RenewRequest{
		WorkspaceId: ws.WorkspaceId,
		DeviceId:    deviceID,
		CsrPem:      mustBuildCSR(t, ws.WorkspaceId, deviceID),
	}
	renewed, err := stack.enrollment.Renew(contextWithMTLSCert(t, enrolled.CertificatePem), renewReq)
	if err != nil {
		t.Fatalf("first renew failed: %v", err)
	}

	_, err = stack.enrollment.Renew(contextWithMTLSCert(t, enrolled.CertificatePem), renewReq)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied when renewing with old fingerprint, got err=%v code=%v", err, status.Code(err))
	}

	_, err = stack.enrollment.Renew(contextWithMTLSCert(t, renewed.CertificatePem), renewReq)
	if err != nil {
		t.Fatalf("renew with rotated cert should succeed: %v", err)
	}
}

type integrationStack struct {
	workspace  *workspacegrpc.Handler
	enrollment *enrollmentgrpc.Handler
	devices    *devicegrpc.Handler
}

func newIntegrationStack(t *testing.T) integrationStack {
	t.Helper()

	encryptor, err := workspace.NewAESGCMEncryptor([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewAESGCMEncryptor: %v", err)
	}

	workspaceRepo := workspace.NewMemoryRepository()
	workspaceSvc := workspace.NewService(workspaceRepo, encryptor)

	tokenRepo := token.NewMemoryRepository()
	tokenSvc := token.NewService(tokenRepo)

	deviceRepo := device.NewMemoryRepository()
	enrollSvc := enrollment.NewService(workspaceRepo, tokenSvc, deviceRepo, encryptor, enrollment.NewLocalSigner(), nil)

	revocationSvc := revocation.NewService(newMemoryRevocationRepo(), revocation.NewCache(), revocation.NewBroker())

	return integrationStack{
		workspace:  workspacegrpc.NewHandler(workspaceSvc, tokenSvc),
		enrollment: enrollmentgrpc.NewHandler(enrollSvc, workspaceSvc, deviceRepo, revocationSvc, true),
		devices:    devicegrpc.NewHandler(deviceRepo, revocationSvc),
	}
}

func (s integrationStack) createWorkspace(t *testing.T) *controlplanev1.CreateWorkspaceResponse {
	t.Helper()
	out, err := s.workspace.CreateWorkspace(context.Background(), &controlplanev1.CreateWorkspaceRequest{
		DisplayName: "integration",
	})
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	return out
}

func (s integrationStack) createToken(
	t *testing.T,
	workspaceID string,
	tokenType controlplanev1.EnrollTokenType,
) *controlplanev1.CreateEnrollTokenResponse {
	t.Helper()
	out, err := s.workspace.CreateEnrollToken(context.Background(), &controlplanev1.CreateEnrollTokenRequest{
		WorkspaceId: workspaceID,
		Type:        tokenType,
		ExpiresAt:   timestamppb.New(time.Now().UTC().Add(15 * time.Minute)),
	})
	if err != nil {
		t.Fatalf("CreateEnrollToken: %v", err)
	}
	return out
}

func (s integrationStack) enroll(
	t *testing.T,
	workspaceID, rawToken string,
	tokenType controlplanev1.EnrollTokenType,
	deviceID string,
) *controlplanev1.EnrollResponse {
	t.Helper()
	out, err := s.enrollment.Enroll(context.Background(), &controlplanev1.EnrollRequest{
		WorkspaceId:    workspaceID,
		BootstrapToken: rawToken,
		Type:           tokenType,
		CsrPem:         mustBuildCSR(t, workspaceID, deviceID),
		DeviceId:       deviceID,
	})
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	return out
}

func contextWithMTLSCert(t *testing.T, certPEM string) context.Context {
	t.Helper()
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		t.Fatalf("decode cert pem: empty block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}

	tlsInfo := credentials.TLSInfo{
		State: tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{cert},
		},
	}
	return peer.NewContext(context.Background(), &peer.Peer{AuthInfo: tlsInfo})
}

func mustBuildCSR(t *testing.T, workspaceID, deviceID string) string {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	uri, err := url.Parse("ztna://" + workspaceID + "/" + deviceID)
	if err != nil {
		t.Fatalf("Parse URI: %v", err)
	}

	req := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   deviceID,
			Organization: []string{workspaceID},
		},
		URIs:     []*url.URL{uri},
		DNSNames: []string{deviceID + "." + workspaceID},
	}

	der, err := x509.CreateCertificateRequest(rand.Reader, req, priv)
	if err != nil {
		t.Fatalf("CreateCertificateRequest: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}))
}

type memoryRevocationRepo struct {
	entries map[string]revocation.Entry
}

func newMemoryRevocationRepo() *memoryRevocationRepo {
	return &memoryRevocationRepo{entries: map[string]revocation.Entry{}}
}

func (r *memoryRevocationRepo) Insert(_ context.Context, in revocation.Entry) error {
	k := in.WorkspaceID + ":" + in.CertFingerprint
	if _, ok := r.entries[k]; ok {
		return revocation.ErrAlreadyRevoked
	}
	r.entries[k] = in
	return nil
}

func (r *memoryRevocationRepo) Exists(_ context.Context, workspaceID, fingerprint string) (bool, error) {
	_, ok := r.entries[workspaceID+":"+fingerprint]
	return ok, nil
}
