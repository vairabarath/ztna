package enrollment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/igris/ztna/controller/internal/device"
	"github.com/igris/ztna/controller/internal/pki"
	"github.com/igris/ztna/controller/internal/storage"
	"github.com/igris/ztna/controller/internal/token"
	"github.com/igris/ztna/controller/internal/workspace"
)

type KeyDecryptor interface {
	Decrypt(ciphertext []byte) ([]byte, error)
}

type service struct {
	workspaceRepo workspace.Repository
	tokenSvc      token.Service
	deviceRepo    device.Repository
	decryptor     KeyDecryptor
	signer        Signer
	txRunner      storage.TxRunner
	now           func() time.Time
}

func NewService(
	workspaceRepo workspace.Repository,
	tokenSvc token.Service,
	deviceRepo device.Repository,
	decryptor KeyDecryptor,
	signer Signer,
	txRunner storage.TxRunner,
) Service {
	return &service{
		workspaceRepo: workspaceRepo,
		tokenSvc:      tokenSvc,
		deviceRepo:    deviceRepo,
		decryptor:     decryptor,
		signer:        signer,
		txRunner:      txRunner,
		now:           time.Now,
	}
}

func (s *service) Enroll(ctx context.Context, in EnrollInput) (EnrollOutput, error) {
	if s.txRunner == nil {
		return s.enrollOnce(ctx, in)
	}

	var out EnrollOutput
	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		out, err = s.enrollOnce(txCtx, in)
		return err
	}); err != nil {
		return EnrollOutput{}, err
	}
	return out, nil
}

func (s *service) enrollOnce(ctx context.Context, in EnrollInput) (EnrollOutput, error) {
	if in.WorkspaceID == "" || in.BootstrapToken == "" || in.CSRPEM == "" || in.DeviceID == "" || in.Type == "" {
		return EnrollOutput{}, ErrInvalidInput
	}

	ws, err := s.workspaceRepo.GetByID(ctx, in.WorkspaceID)
	if err != nil {
		if errors.Is(err, workspace.ErrNotFound) {
			return EnrollOutput{}, ErrWorkspaceNotFound
		}
		return EnrollOutput{}, fmt.Errorf("get workspace: %w", err)
	}

	valid, err := s.tokenSvc.Validate(ctx, token.ValidateInput{
		WorkspaceID: in.WorkspaceID,
		Type:        in.Type,
		RawToken:    in.BootstrapToken,
	})
	if err != nil {
		return EnrollOutput{}, err
	}

	if _, err := ValidateCSR(in.CSRPEM, in.WorkspaceID, in.DeviceID); err != nil {
		return EnrollOutput{}, err
	}

	caKeyPEMBytes, err := s.decryptor.Decrypt(ws.CAPrivateKeyEncrypted)
	if err != nil {
		return EnrollOutput{}, fmt.Errorf("decrypt workspace ca key: %w", err)
	}

	expiresAt := s.now().UTC().Add(pki.DeviceCertLifetime)
	certPEM, fingerprint, err := s.signer.Sign(SignInput{
		CSRPEM: in.CSRPEM,
		Profile: Profile{
			WorkspaceID: in.WorkspaceID,
			DeviceID:    in.DeviceID,
			NotAfter:    expiresAt,
		},
		CACertPEM:       ws.CACertPEM,
		CAPrivateKeyPEM: string(caKeyPEMBytes),
	})
	if err != nil {
		return EnrollOutput{}, err
	}

	if err := s.deviceRepo.Upsert(ctx, device.Device{
		WorkspaceID:      in.WorkspaceID,
		DeviceID:         in.DeviceID,
		CertFingerprint:  fingerprint,
		Status:           "active",
		LastSeenUnixTime: s.now().UTC().Unix(),
	}); err != nil {
		return EnrollOutput{}, fmt.Errorf("upsert device: %w", err)
	}

	if err := s.tokenSvc.MarkUsed(ctx, valid.TokenID); err != nil {
		return EnrollOutput{}, fmt.Errorf("mark token used: %w", err)
	}

	return EnrollOutput{
		CertificatePEM:  certPEM,
		CACertPEM:       ws.CACertPEM,
		CertFingerprint: fingerprint,
		ExpiresAt:       expiresAt,
	}, nil
}

func (s *service) Renew(ctx context.Context, in RenewInput) (RenewOutput, error) {
	if in.WorkspaceID == "" || in.CSRPEM == "" || in.DeviceID == "" {
		return RenewOutput{}, ErrInvalidInput
	}

	ws, err := s.workspaceRepo.GetByID(ctx, in.WorkspaceID)
	if err != nil {
		if errors.Is(err, workspace.ErrNotFound) {
			return RenewOutput{}, ErrWorkspaceNotFound
		}
		return RenewOutput{}, fmt.Errorf("get workspace: %w", err)
	}

	if _, err := ValidateCSR(in.CSRPEM, in.WorkspaceID, in.DeviceID); err != nil {
		return RenewOutput{}, err
	}

	caKeyPEMBytes, err := s.decryptor.Decrypt(ws.CAPrivateKeyEncrypted)
	if err != nil {
		return RenewOutput{}, fmt.Errorf("decrypt workspace ca key: %w", err)
	}

	expiresAt := s.now().UTC().Add(pki.DeviceCertLifetime)
	certPEM, fingerprint, err := s.signer.Sign(SignInput{
		CSRPEM: in.CSRPEM,
		Profile: Profile{
			WorkspaceID: in.WorkspaceID,
			DeviceID:    in.DeviceID,
			NotAfter:    expiresAt,
		},
		CACertPEM:       ws.CACertPEM,
		CAPrivateKeyPEM: string(caKeyPEMBytes),
	})
	if err != nil {
		return RenewOutput{}, err
	}

	if err := s.deviceRepo.Upsert(ctx, device.Device{
		WorkspaceID:      in.WorkspaceID,
		DeviceID:         in.DeviceID,
		CertFingerprint:  fingerprint,
		Status:           "active",
		LastSeenUnixTime: s.now().UTC().Unix(),
	}); err != nil {
		return RenewOutput{}, fmt.Errorf("upsert device: %w", err)
	}

	return RenewOutput{
		CertificatePEM:  certPEM,
		CACertPEM:       ws.CACertPEM,
		CertFingerprint: fingerprint,
		ExpiresAt:       expiresAt,
	}, nil
}
