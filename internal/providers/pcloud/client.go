package pcloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/svosadtsia/csync/internal/config"
	"github.com/svosadtsia/csync/pkg/utils"
)

// Client represents a pCloud client
type Client struct {
	config     *config.PCloudConfig
	httpClient *http.Client
	authToken  string
}

// APIResponse represents a generic pCloud API response
type APIResponse struct {
	Result    int    `json:"result"`
	Error     string `json:"error,omitempty"`
	AuthToken string `json:"auth,omitempty"`
}

// FileResponse represents a file operation response
type FileResponse struct {
	APIResponse
	FileID   int64       `json:"fileid,omitempty"`
	Metadata interface{} `json:"metadata,omitempty"`
}

// FolderResponse represents a folder operation response
type FolderResponse struct {
	APIResponse
	FolderID int64       `json:"folderid,omitempty"`
	Metadata interface{} `json:"metadata,omitempty"`
}

// NewClient creates a new pCloud client
func NewClient(cfg *config.PCloudConfig) (*Client, error) {
	client := &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Authenticate
	if err := client.authenticate(); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	return client, nil
}

// authenticate performs authentication with pCloud
func (c *Client) authenticate() error {
	// pCloud uses /userinfo endpoint for authentication with credentials
	url := fmt.Sprintf("%s/userinfo", c.config.APIHost)

	data := map[string]string{
		"username": c.config.Username,
		"password": c.config.Password,
	}

	resp, err := c.makeRequest("POST", url, data, nil)
	if err != nil {
		return fmt.Errorf("authentication request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	utils.LogDebug("pCloud auth response: %s", string(body))

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("failed to decode authentication response: %w", err)
	}

	if apiResp.Result != 0 {
		return fmt.Errorf("authentication failed: %s", apiResp.Error)
	}

	// For pCloud, successful userinfo call means we're authenticated
	// We'll use username/password for subsequent requests
	c.authToken = "authenticated" // Just a flag to indicate successful auth
	utils.LogVerbose("Successfully authenticated with pCloud (%s)", c.config.Username)
	return nil
}

// Sync syncs a directory to pCloud
func (c *Client) Sync(ctx context.Context, sourcePath string) error {
	utils.LogVerbose("Starting pCloud sync from: %s", sourcePath)

	return filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing path %s: %w", path, err)
		}

		relPath, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		if relPath == "." {
			return nil
		}

		if utils.ShouldIgnore(relPath, []string{".git/", ".DS_Store", "Thumbs.db"}) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			return c.createFolder(ctx, relPath)
		}

		return c.uploadFile(ctx, path, relPath)
	})
}

// DryRun shows what would be synced without actually syncing
func (c *Client) DryRun(ctx context.Context, sourcePath string) error {
	utils.LogVerbose("DRY RUN: pCloud sync from: %s", sourcePath)

	return filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing path %s: %w", path, err)
		}

		relPath, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		if relPath == "." {
			return nil
		}

		if utils.ShouldIgnore(relPath, []string{".git/", ".DS_Store", "Thumbs.db"}) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			utils.LogInfo("→ %s/ (folder)", relPath)
		} else {
			utils.LogInfo("→ %s (%d bytes)", relPath, info.Size())
		}

		return nil
	})
}

// createFolder creates a folder in pCloud
func (c *Client) createFolder(ctx context.Context, folderPath string) error {
	parts := strings.Split(folderPath, string(filepath.Separator))

	parentFolderID := c.config.FolderID
	if parentFolderID == "" {
		parentFolderID = "0" // Root folder
	}

	for _, part := range parts {
		if part == "" {
			continue
		}

		// Check if folder exists
		folderID, err := c.findFolder(ctx, part, parentFolderID)
		if err != nil {
			return fmt.Errorf("failed to check for existing folder: %w", err)
		}

		if folderID != "" {
			parentFolderID = folderID
			continue
		}

		// Create the folder
		url := fmt.Sprintf("%s/createfolder", c.config.APIHost)
		data := map[string]string{
			"username": c.config.Username,
			"password": c.config.Password,
			"name":     part,
			"folderid": parentFolderID,
		}

		resp, err := c.makeRequest("POST", url, data, nil)
		if err != nil {
			return fmt.Errorf("failed to create folder request: %w", err)
		}
		defer resp.Body.Close()

		var folderResp FolderResponse
		if err := json.NewDecoder(resp.Body).Decode(&folderResp); err != nil {
			return fmt.Errorf("failed to decode folder response: %w", err)
		}

		if folderResp.Result != 0 {
			return fmt.Errorf("failed to create folder %s: %s", part, folderResp.Error)
		}

		utils.LogVerbose("Created folder: %s", part)
		parentFolderID = strconv.FormatInt(folderResp.FolderID, 10)
	}

	return nil
}

// uploadFile uploads a file to pCloud
func (c *Client) uploadFile(ctx context.Context, localPath, remotePath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Determine parent folder using destination path
	var targetPath string
	if c.config.DestinationPath != "" {
		// Combine destination path with the directory part of remotePath
		dir := filepath.Dir(remotePath)
		if dir == "." {
			targetPath = c.config.DestinationPath
		} else {
			targetPath = filepath.Join(c.config.DestinationPath, dir)
		}
	} else {
		targetPath = filepath.Dir(remotePath)
	}

	// Create the target directory structure
	if err := c.createFolder(ctx, targetPath); err != nil {
		return fmt.Errorf("failed to create target folders: %w", err)
	}

	// Get the folder ID for the target path
	// We pass an empty path to getFolderID since targetPath is already the full path
	parentFolderID, err := c.getFolderIDDirect(ctx, targetPath)
	if err != nil {
		return fmt.Errorf("failed to get target folder ID: %w", err)
	}

	// Upload the file
	url := fmt.Sprintf("%s/uploadfile", c.config.APIHost)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add authentication credentials
	writer.WriteField("username", c.config.Username)
	writer.WriteField("password", c.config.Password)
	writer.WriteField("folderid", parentFolderID)

	// Add file
	fileName := filepath.Base(remotePath)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("failed to copy file data: %w", err)
	}

	writer.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", url, &body)
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	var fileResp FileResponse
	if err := json.NewDecoder(resp.Body).Decode(&fileResp); err != nil {
		return fmt.Errorf("failed to decode upload response: %w", err)
	}

	if fileResp.Result != 0 {
		return fmt.Errorf("upload failed: %s", fileResp.Error)
	}

	utils.LogInfo("✓ %s (%d bytes)", remotePath, fileInfo.Size())
	return nil
}

// findFolder finds a folder by name in the given parent
func (c *Client) findFolder(ctx context.Context, name, parentFolderID string) (string, error) {
	url := fmt.Sprintf("%s/listfolder", c.config.APIHost)
	data := map[string]string{
		"username": c.config.Username,
		"password": c.config.Password,
		"folderid": parentFolderID,
	}

	resp, err := c.makeRequest("GET", url, data, nil)
	if err != nil {
		return "", fmt.Errorf("failed to list folder: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode folder list: %w", err)
	}

	if resultCode, ok := result["result"].(float64); !ok || resultCode != 0 {
		return "", fmt.Errorf("folder list failed")
	}

	// Look for folder in contents
	if metadata, ok := result["metadata"].(map[string]interface{}); ok {
		if contents, ok := metadata["contents"].([]interface{}); ok {
			for _, item := range contents {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if itemName, ok := itemMap["name"].(string); ok && itemName == name {
						if isFolder, ok := itemMap["isfolder"].(bool); ok && isFolder {
							if folderID, ok := itemMap["folderid"].(float64); ok {
								return strconv.FormatInt(int64(folderID), 10), nil
							}
						}
					}
				}
			}
		}
	}

	return "", nil
}

// getFolderID gets the folder ID for a given path
func (c *Client) getFolderID(ctx context.Context, folderPath string) (string, error) {
	// Start from the configured destination path if specified
	var basePath string
	if c.config.DestinationPath != "" {
		basePath = c.config.DestinationPath
	}

	// Combine base path with relative folder path
	if basePath != "" {
		folderPath = filepath.Join(basePath, folderPath)
	}

	parts := strings.Split(strings.Trim(folderPath, "/"), "/")

	parentFolderID := c.config.FolderID
	if parentFolderID == "" {
		parentFolderID = "0" // Root folder
	}

	for _, part := range parts {
		if part == "" {
			continue
		}

		folderID, err := c.findFolder(ctx, part, parentFolderID)
		if err != nil {
			return "", err
		}

		if folderID == "" {
			return "", fmt.Errorf("folder not found: %s", part)
		}

		parentFolderID = folderID
	}

	return parentFolderID, nil
}

// makeRequest makes an HTTP request to the pCloud API
func (c *Client) makeRequest(method, url string, data map[string]string, body io.Reader) (*http.Response, error) {
	if method == "GET" && data != nil {
		// Add query parameters for GET requests
		req, err := http.NewRequest(method, url, nil)
		if err != nil {
			return nil, err
		}

		q := req.URL.Query()
		for key, value := range data {
			q.Add(key, value)
		}
		req.URL.RawQuery = q.Encode()

		return c.httpClient.Do(req)
	}

	// For POST requests with form data
	if method == "POST" && data != nil && body == nil {
		formData := make([]string, 0, len(data))
		for key, value := range data {
			formData = append(formData, fmt.Sprintf("%s=%s", key, value))
		}
		body = strings.NewReader(strings.Join(formData, "&"))
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	if method == "POST" && body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	return c.httpClient.Do(req)
}

// getFolderIDDirect gets the folder ID for a given absolute path (without adding destination path)
func (c *Client) getFolderIDDirect(ctx context.Context, folderPath string) (string, error) {
	parts := strings.Split(strings.Trim(folderPath, "/"), "/")

	parentFolderID := c.config.FolderID
	if parentFolderID == "" {
		parentFolderID = "0" // Root folder
	}

	for _, part := range parts {
		if part == "" {
			continue
		}

		folderID, err := c.findFolder(ctx, part, parentFolderID)
		if err != nil {
			return "", err
		}

		if folderID == "" {
			return "", fmt.Errorf("folder not found: %s", part)
		}

		parentFolderID = folderID
	}

	return parentFolderID, nil
}
