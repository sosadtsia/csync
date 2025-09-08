package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"

	"github.com/svosadtsia/csync/internal/config"
	"github.com/svosadtsia/csync/internal/scanner"
)

// GoogleDriveProvider implements the Provider interface for Google Drive
type GoogleDriveProvider struct {
	service  *drive.Service
	config   *config.GoogleDriveConfig
	folderID string
}

// NewGoogleDriveProvider creates a new Google Drive provider
func NewGoogleDriveProvider(cfg *config.GoogleDriveConfig) (*GoogleDriveProvider, error) {
	ctx := context.Background()

	// Read credentials file
	credentials, err := os.ReadFile(cfg.CredentialsPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials file: %w", err)
	}

	// Parse credentials
	oauthConfig, err := google.ConfigFromJSON(credentials, cfg.Scopes...)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file: %w", err)
	}

	// Get OAuth2 client
	client, err := getClient(oauthConfig, cfg.TokenPath)
	if err != nil {
		return nil, fmt.Errorf("unable to get OAuth2 client: %w", err)
	}

	// Create Drive service
	service, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Drive client: %w", err)
	}

	provider := &GoogleDriveProvider{
		service:  service,
		config:   cfg,
		folderID: cfg.FolderID,
	}

	// If no folder ID specified, use root
	if provider.folderID == "" {
		provider.folderID = "root"
	}

	return provider, nil
}

// Name returns the provider name
func (p *GoogleDriveProvider) Name() string {
	return "Google Drive"
}

// Upload uploads a file to Google Drive
func (p *GoogleDriveProvider) Upload(ctx context.Context, file scanner.FileInfo, remotePath string) error {
	// Open the local file
	localFile, err := os.Open(file.AbsolutePath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	// Prepare the drive file metadata
	driveFile := &drive.File{
		Name: filepath.Base(remotePath),
	}

	// Set parent folder if specified
	parentID, err := p.ensureParentFolders(ctx, remotePath)
	if err != nil {
		return fmt.Errorf("failed to ensure parent folders: %w", err)
	}
	if parentID != "" {
		driveFile.Parents = []string{parentID}
	}

	// Add metadata if configured
	if len(p.config.Metadata) > 0 {
		properties := make(map[string]string)
		for k, v := range p.config.Metadata {
			properties[k] = v
		}
		driveFile.Properties = properties
	}

	// Check if file already exists
	existingFileID, err := p.findFile(ctx, filepath.Base(remotePath), parentID)
	if err != nil {
		return fmt.Errorf("failed to check existing file: %w", err)
	}

	if existingFileID != "" {
		// Update existing file
		_, err = p.service.Files.Update(existingFileID, driveFile).
			Context(ctx).
			Media(localFile).
			Do()
		if err != nil {
			return fmt.Errorf("failed to update file: %w", err)
		}
	} else {
		// Create new file
		_, err = p.service.Files.Create(driveFile).
			Context(ctx).
			Media(localFile).
			Do()
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
	}

	return nil
}

// CreateFolder creates a folder in Google Drive
func (p *GoogleDriveProvider) CreateFolder(ctx context.Context, remotePath string) error {
	_, err := p.ensureParentFolders(ctx, remotePath+"/dummy")
	return err
}

// FileExists checks if a file exists in Google Drive
func (p *GoogleDriveProvider) FileExists(ctx context.Context, remotePath string) (bool, error) {
	parentID, err := p.getParentFolderID(ctx, remotePath)
	if err != nil {
		return false, err
	}

	fileName := filepath.Base(remotePath)
	fileID, err := p.findFile(ctx, fileName, parentID)
	if err != nil {
		return false, err
	}

	return fileID != "", nil
}

// GetFileInfo retrieves information about a remote file
func (p *GoogleDriveProvider) GetFileInfo(ctx context.Context, remotePath string) (*RemoteFileInfo, error) {
	parentID, err := p.getParentFolderID(ctx, remotePath)
	if err != nil {
		return nil, err
	}

	fileName := filepath.Base(remotePath)
	fileID, err := p.findFile(ctx, fileName, parentID)
	if err != nil {
		return nil, err
	}

	if fileID == "" {
		return nil, fmt.Errorf("file not found: %s", remotePath)
	}

	file, err := p.service.Files.Get(fileID).
		Context(ctx).
		Fields("id,name,size,md5Checksum,modifiedTime").
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	return &RemoteFileInfo{
		Path:     remotePath,
		Size:     file.Size,
		MD5Hash:  file.Md5Checksum,
		Modified: file.ModifiedTime,
	}, nil
}

// Delete removes a file or folder from Google Drive
func (p *GoogleDriveProvider) Delete(ctx context.Context, remotePath string) error {
	parentID, err := p.getParentFolderID(ctx, remotePath)
	if err != nil {
		return err
	}

	fileName := filepath.Base(remotePath)
	fileID, err := p.findFile(ctx, fileName, parentID)
	if err != nil {
		return err
	}

	if fileID == "" {
		return fmt.Errorf("file not found: %s", remotePath)
	}

	err = p.service.Files.Delete(fileID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// ensureParentFolders ensures all parent directories exist for a given path
func (p *GoogleDriveProvider) ensureParentFolders(ctx context.Context, remotePath string) (string, error) {
	dir := filepath.Dir(remotePath)
	if dir == "." || dir == "/" {
		return p.folderID, nil
	}

	parentID := p.folderID
	parts := strings.Split(filepath.ToSlash(dir), "/")

	for _, part := range parts {
		if part == "" {
			continue
		}

		// Check if folder already exists
		folderID, err := p.findFile(ctx, part, parentID)
		if err != nil {
			return "", fmt.Errorf("failed to check folder existence: %w", err)
		}

		if folderID == "" {
			// Create folder
			folder := &drive.File{
				Name:     part,
				MimeType: "application/vnd.google-apps.folder",
				Parents:  []string{parentID},
			}

			createdFolder, err := p.service.Files.Create(folder).Context(ctx).Do()
			if err != nil {
				return "", fmt.Errorf("failed to create folder %s: %w", part, err)
			}

			folderID = createdFolder.Id
		}

		parentID = folderID
	}

	return parentID, nil
}

// getParentFolderID gets the parent folder ID for a given path
func (p *GoogleDriveProvider) getParentFolderID(ctx context.Context, remotePath string) (string, error) {
	dir := filepath.Dir(remotePath)
	if dir == "." || dir == "/" {
		return p.folderID, nil
	}

	parentID := p.folderID
	parts := strings.Split(filepath.ToSlash(dir), "/")

	for _, part := range parts {
		if part == "" {
			continue
		}

		folderID, err := p.findFile(ctx, part, parentID)
		if err != nil {
			return "", fmt.Errorf("failed to find folder %s: %w", part, err)
		}

		if folderID == "" {
			return "", fmt.Errorf("folder not found: %s", part)
		}

		parentID = folderID
	}

	return parentID, nil
}

// findFile finds a file or folder by name in the specified parent folder
func (p *GoogleDriveProvider) findFile(ctx context.Context, name, parentID string) (string, error) {
	query := fmt.Sprintf("name='%s' and '%s' in parents and trashed=false", name, parentID)

	fileList, err := p.service.Files.List().
		Context(ctx).
		Q(query).
		Fields("files(id,name)").
		Do()
	if err != nil {
		return "", fmt.Errorf("failed to search for file: %w", err)
	}

	if len(fileList.Files) == 0 {
		return "", nil // File not found
	}

	return fileList.Files[0].Id, nil
}

// getClient retrieves an OAuth2 client
func getClient(config *oauth2.Config, tokenFile string) (*http.Client, error) {
	token, err := tokenFromFile(tokenFile)
	if err != nil {
		token, err = getTokenFromWeb(config)
		if err != nil {
			return nil, fmt.Errorf("unable to get token from web: %w", err)
		}
		if err := saveToken(tokenFile, token); err != nil {
			return nil, fmt.Errorf("unable to save token: %w", err)
		}
	}

	return config.Client(context.Background(), token), nil
}

// getTokenFromWeb requests a token from the web
func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the authorization code:\n%v\n", authURL)

	var authCode string
	fmt.Print("Enter authorization code: ")
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("unable to read authorization code: %w", err)
	}

	token, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %w", err)
	}

	return token, nil
}

// tokenFromFile retrieves a token from a local file
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	token := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(token)
	return token, err
}

// saveToken saves a token to a file path
func saveToken(path string, token *oauth2.Token) error {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to cache oauth token: %w", err)
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(token)
}
