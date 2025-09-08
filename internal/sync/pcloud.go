package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/svosadtsia/csync/internal/config"
	"github.com/svosadtsia/csync/internal/scanner"
)

// PCloudProvider implements the Provider interface for pCloud
type PCloudProvider struct {
	client   *http.Client
	config   *config.PCloudConfig
	folderID string
	auth     string // Authentication token
}

// PCloudResponse represents a generic pCloud API response
type PCloudResponse struct {
	Result   int             `json:"result"`
	Error    string          `json:"error,omitempty"`
	Auth     string          `json:"auth,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// PCloudFileMetadata represents file metadata from pCloud
type PCloudFileMetadata struct {
	FileID   int64  `json:"fileid"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Hash     string `json:"hash"`
	Modified string `json:"modified"`
	IsFolder bool   `json:"isfolder"`
	FolderID int64  `json:"parentfolderid"`
}

// PCloudFolderMetadata represents folder contents from pCloud
type PCloudFolderContents struct {
	Contents []PCloudFileMetadata `json:"contents"`
}

// NewPCloudProvider creates a new pCloud provider
func NewPCloudProvider(cfg *config.PCloudConfig) (*PCloudProvider, error) {
	provider := &PCloudProvider{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		config:   cfg,
		folderID: cfg.FolderID,
	}

	// Authenticate
	if err := provider.authenticate(); err != nil {
		return nil, fmt.Errorf("failed to authenticate with pCloud: %w", err)
	}

	// If no folder ID specified, use root (0)
	if provider.folderID == "" {
		provider.folderID = "0"
	}

	return provider, nil
}

// Name returns the provider name
func (p *PCloudProvider) Name() string {
	return "pCloud"
}

// authenticate performs authentication with pCloud
func (p *PCloudProvider) authenticate() error {
	data := url.Values{}
	data.Set("username", p.config.Username)
	data.Set("password", p.config.Password)

	resp, err := p.client.PostForm(p.config.APIHost+"/userinfo", data)
	if err != nil {
		return fmt.Errorf("authentication request failed: %w", err)
	}
	defer resp.Body.Close()

	var authResp PCloudResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return fmt.Errorf("failed to decode authentication response: %w", err)
	}

	if authResp.Result != 0 {
		return fmt.Errorf("authentication failed: %s", authResp.Error)
	}

	p.auth = authResp.Auth
	return nil
}

// Upload uploads a file to pCloud
func (p *PCloudProvider) Upload(ctx context.Context, file scanner.FileInfo, remotePath string) error {
	// Ensure parent folders exist
	parentFolderID, err := p.ensureParentFolders(ctx, remotePath)
	if err != nil {
		return fmt.Errorf("failed to ensure parent folders: %w", err)
	}

	// Open local file
	localFile, err := os.Open(file.AbsolutePath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add auth
	writer.WriteField("auth", p.auth)
	writer.WriteField("folderid", parentFolderID)
	writer.WriteField("filename", filepath.Base(remotePath))

	// Add file
	fileWriter, err := writer.CreateFormFile("file", filepath.Base(remotePath))
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(fileWriter, localFile); err != nil {
		return fmt.Errorf("failed to copy file data: %w", err)
	}

	writer.Close()

	// Make request
	req, err := http.NewRequestWithContext(ctx, "POST", p.config.APIHost+"/uploadfile", &buf)
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	var uploadResp PCloudResponse
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		return fmt.Errorf("failed to decode upload response: %w", err)
	}

	if uploadResp.Result != 0 {
		return fmt.Errorf("upload failed: %s", uploadResp.Error)
	}

	return nil
}

// CreateFolder creates a folder in pCloud
func (p *PCloudProvider) CreateFolder(ctx context.Context, remotePath string) error {
	_, err := p.ensureParentFolders(ctx, remotePath+"/dummy")
	return err
}

// FileExists checks if a file exists in pCloud
func (p *PCloudProvider) FileExists(ctx context.Context, remotePath string) (bool, error) {
	parentFolderID, err := p.getParentFolderID(ctx, remotePath)
	if err != nil {
		return false, err
	}

	fileName := filepath.Base(remotePath)
	_, err = p.findFile(ctx, fileName, parentFolderID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// GetFileInfo retrieves information about a remote file
func (p *PCloudProvider) GetFileInfo(ctx context.Context, remotePath string) (*RemoteFileInfo, error) {
	parentFolderID, err := p.getParentFolderID(ctx, remotePath)
	if err != nil {
		return nil, err
	}

	fileName := filepath.Base(remotePath)
	metadata, err := p.findFile(ctx, fileName, parentFolderID)
	if err != nil {
		return nil, err
	}

	return &RemoteFileInfo{
		Path:     remotePath,
		Size:     metadata.Size,
		MD5Hash:  metadata.Hash,
		Modified: metadata.Modified,
	}, nil
}

// Delete removes a file or folder from pCloud
func (p *PCloudProvider) Delete(ctx context.Context, remotePath string) error {
	parentFolderID, err := p.getParentFolderID(ctx, remotePath)
	if err != nil {
		return err
	}

	fileName := filepath.Base(remotePath)
	metadata, err := p.findFile(ctx, fileName, parentFolderID)
	if err != nil {
		return err
	}

	// Prepare delete request
	data := url.Values{}
	data.Set("auth", p.auth)

	var endpoint string
	if metadata.IsFolder {
		endpoint = "/deletefolder"
		data.Set("folderid", strconv.FormatInt(metadata.FileID, 10))
	} else {
		endpoint = "/deletefile"
		data.Set("fileid", strconv.FormatInt(metadata.FileID, 10))
	}

	resp, err := p.client.PostForm(p.config.APIHost+endpoint, data)
	if err != nil {
		return fmt.Errorf("delete request failed: %w", err)
	}
	defer resp.Body.Close()

	var deleteResp PCloudResponse
	if err := json.NewDecoder(resp.Body).Decode(&deleteResp); err != nil {
		return fmt.Errorf("failed to decode delete response: %w", err)
	}

	if deleteResp.Result != 0 {
		return fmt.Errorf("delete failed: %s", deleteResp.Error)
	}

	return nil
}

// ensureParentFolders ensures all parent directories exist for a given path
func (p *PCloudProvider) ensureParentFolders(ctx context.Context, remotePath string) (string, error) {
	dir := filepath.Dir(remotePath)
	if dir == "." || dir == "/" {
		return p.folderID, nil
	}

	parentFolderID := p.folderID
	parts := strings.Split(filepath.ToSlash(dir), "/")

	for _, part := range parts {
		if part == "" {
			continue
		}

		// Check if folder already exists
		metadata, err := p.findFile(ctx, part, parentFolderID)
		if err != nil && !strings.Contains(err.Error(), "not found") {
			return "", fmt.Errorf("failed to check folder existence: %w", err)
		}

		if err != nil && strings.Contains(err.Error(), "not found") {
			// Create folder
			folderID, err := p.createFolder(ctx, part, parentFolderID)
			if err != nil {
				return "", fmt.Errorf("failed to create folder %s: %w", part, err)
			}
			parentFolderID = folderID
		} else {
			if !metadata.IsFolder {
				return "", fmt.Errorf("path conflict: %s is a file, not a folder", part)
			}
			parentFolderID = strconv.FormatInt(metadata.FileID, 10)
		}
	}

	return parentFolderID, nil
}

// getParentFolderID gets the parent folder ID for a given path
func (p *PCloudProvider) getParentFolderID(ctx context.Context, remotePath string) (string, error) {
	dir := filepath.Dir(remotePath)
	if dir == "." || dir == "/" {
		return p.folderID, nil
	}

	parentFolderID := p.folderID
	parts := strings.Split(filepath.ToSlash(dir), "/")

	for _, part := range parts {
		if part == "" {
			continue
		}

		metadata, err := p.findFile(ctx, part, parentFolderID)
		if err != nil {
			return "", fmt.Errorf("failed to find folder %s: %w", part, err)
		}

		if !metadata.IsFolder {
			return "", fmt.Errorf("path conflict: %s is a file, not a folder", part)
		}

		parentFolderID = strconv.FormatInt(metadata.FileID, 10)
	}

	return parentFolderID, nil
}

// createFolder creates a new folder in pCloud
func (p *PCloudProvider) createFolder(ctx context.Context, name, parentFolderID string) (string, error) {
	data := url.Values{}
	data.Set("auth", p.auth)
	data.Set("folderid", parentFolderID)
	data.Set("name", name)

	resp, err := p.client.PostForm(p.config.APIHost+"/createfolder", data)
	if err != nil {
		return "", fmt.Errorf("create folder request failed: %w", err)
	}
	defer resp.Body.Close()

	var createResp struct {
		Result   int    `json:"result"`
		Error    string `json:"error,omitempty"`
		Metadata struct {
			FolderID int64 `json:"folderid"`
		} `json:"metadata"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return "", fmt.Errorf("failed to decode create folder response: %w", err)
	}

	if createResp.Result != 0 {
		return "", fmt.Errorf("create folder failed: %s", createResp.Error)
	}

	return strconv.FormatInt(createResp.Metadata.FolderID, 10), nil
}

// findFile finds a file or folder by name in the specified parent folder
func (p *PCloudProvider) findFile(ctx context.Context, name, parentFolderID string) (*PCloudFileMetadata, error) {
	data := url.Values{}
	data.Set("auth", p.auth)
	data.Set("folderid", parentFolderID)

	resp, err := p.client.PostForm(p.config.APIHost+"/listfolder", data)
	if err != nil {
		return nil, fmt.Errorf("list folder request failed: %w", err)
	}
	defer resp.Body.Close()

	var listResp struct {
		Result   int                  `json:"result"`
		Error    string               `json:"error,omitempty"`
		Metadata PCloudFolderContents `json:"metadata"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode list folder response: %w", err)
	}

	if listResp.Result != 0 {
		return nil, fmt.Errorf("list folder failed: %s", listResp.Error)
	}

	// Search for the file/folder by name
	for _, item := range listResp.Metadata.Contents {
		if item.Name == name {
			return &item, nil
		}
	}

	return nil, fmt.Errorf("file not found: %s", name)
}
