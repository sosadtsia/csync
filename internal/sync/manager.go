package sync

import (
	"context"
	"fmt"

	"github.com/svosadtsia/csync/internal/config"
	"github.com/svosadtsia/csync/internal/providers/gdrive"
	"github.com/svosadtsia/csync/internal/providers/pcloud"
)

// Manager handles synchronization operations across different cloud providers
type Manager struct {
	config       *config.Config
	gdriveClient *gdrive.Client
	pcloudClient *pcloud.Client
}

// NewManager creates a new sync manager with the given configuration
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		config: cfg,
	}
}

// SyncToGoogleDrive syncs files to Google Drive
func (m *Manager) SyncToGoogleDrive(ctx context.Context, sourcePath string, dryRun bool) error {
	if m.gdriveClient == nil {
		client, err := gdrive.NewClient(ctx, &m.config.GoogleDrive)
		if err != nil {
			return fmt.Errorf("failed to create Google Drive client: %w", err)
		}
		m.gdriveClient = client
	}

	if dryRun {
		return m.gdriveClient.DryRun(ctx, sourcePath)
	}

	return m.gdriveClient.Sync(ctx, sourcePath)
}

// SyncToPCloud syncs files to pCloud
func (m *Manager) SyncToPCloud(ctx context.Context, sourcePath string, dryRun bool) error {
	if m.pcloudClient == nil {
		client, err := pcloud.NewClient(&m.config.PCloud)
		if err != nil {
			return fmt.Errorf("failed to create pCloud client: %w", err)
		}
		m.pcloudClient = client
	}

	if dryRun {
		return m.pcloudClient.DryRun(ctx, sourcePath)
	}

	return m.pcloudClient.Sync(ctx, sourcePath)
}
