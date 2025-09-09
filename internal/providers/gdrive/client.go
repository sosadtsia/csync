package gdrive

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"

	"github.com/svosadtsia/csync/internal/config"
	"github.com/svosadtsia/csync/pkg/utils"
)

// Client represents a Google Drive client
type Client struct {
	service *drive.Service
	config  *config.GoogleDriveConfig
}

// NewClient creates a new Google Drive client
func NewClient(ctx context.Context, cfg *config.GoogleDriveConfig) (*Client, error) {
	// Read credentials file
	credBytes, err := os.ReadFile(cfg.CredentialsPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials file: %w", err)
	}

	// Parse credentials
	config, err := google.ConfigFromJSON(credBytes, cfg.Scopes...)
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials: %w", err)
	}

	// Get OAuth2 client
	client := getClient(config, cfg.TokenPath)

	// Create Drive service
	service, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to create Drive service: %w", err)
	}

	return &Client{
		service: service,
		config:  cfg,
	}, nil
}

// getClient retrieves a token, saves the token, then returns the generated client
func getClient(config *oauth2.Config, tokFile string) *http.Client {
	// Try to read token from file
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// getTokenFromWeb requests a token from the web, then returns the retrieved token
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// tokenFromFile retrieves a token from a local file
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// saveToken saves a token to a file path
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

// Sync syncs a directory to Google Drive
func (c *Client) Sync(ctx context.Context, sourcePath string) error {
	log.Printf("Starting Google Drive sync from: %s", sourcePath)

	// Walk through the source directory
	return filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing path %s: %w", path, err)
		}

		// Calculate relative path
		relPath, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Skip root directory
		if relPath == "." {
			return nil
		}

		// Check if path should be ignored
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
	log.Printf("DRY RUN: Google Drive sync from: %s", sourcePath)

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
			log.Printf("[DRY RUN] Would create folder: %s", relPath)
		} else {
			log.Printf("[DRY RUN] Would upload file: %s (%d bytes)", relPath, info.Size())
		}

		return nil
	})
}

// createFolder creates a folder in Google Drive
func (c *Client) createFolder(ctx context.Context, folderPath string) error {
	// Split path into components
	parts := strings.Split(folderPath, string(filepath.Separator))

	parentID := c.config.FolderID
	if parentID == "" {
		parentID = "root"
	}

	// Create each folder in the path if it doesn't exist
	for _, part := range parts {
		if part == "" {
			continue
		}

		// Check if folder already exists
		folderID, err := c.findFolder(ctx, part, parentID)
		if err != nil {
			return fmt.Errorf("failed to check for existing folder: %w", err)
		}

		if folderID != "" {
			parentID = folderID
			continue
		}

		// Create the folder
		folder := &drive.File{
			Name:     part,
			MimeType: "application/vnd.google-apps.folder",
			Parents:  []string{parentID},
		}

		createdFolder, err := c.service.Files.Create(folder).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed to create folder %s: %w", part, err)
		}

		log.Printf("Created folder: %s", part)
		parentID = createdFolder.Id
	}

	return nil
}

// uploadFile uploads a file to Google Drive
func (c *Client) uploadFile(ctx context.Context, localPath, remotePath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Determine parent folder
	parentID := c.config.FolderID
	if parentID == "" {
		parentID = "root"
	}

	// If file is in a subdirectory, ensure the folder structure exists
	dir := filepath.Dir(remotePath)
	if dir != "." {
		if err := c.createFolder(ctx, dir); err != nil {
			return fmt.Errorf("failed to create parent folders: %w", err)
		}

		// Get the actual parent folder ID
		folderID, err := c.getFolderID(ctx, dir)
		if err != nil {
			return fmt.Errorf("failed to get parent folder ID: %w", err)
		}
		parentID = folderID
	}

	// Check if file already exists
	fileName := filepath.Base(remotePath)
	existingFileID, err := c.findFile(ctx, fileName, parentID)
	if err != nil {
		return fmt.Errorf("failed to check for existing file: %w", err)
	}

	driveFile := &drive.File{
		Name:    fileName,
		Parents: []string{parentID},
	}

	if existingFileID != "" {
		// Update existing file
		_, err = c.service.Files.Update(existingFileID, driveFile).
			Media(file).
			Context(ctx).
			Do()
		if err != nil {
			return fmt.Errorf("failed to update file: %w", err)
		}
		log.Printf("Updated file: %s", remotePath)
	} else {
		// Create new file
		_, err = c.service.Files.Create(driveFile).
			Media(file).
			Context(ctx).
			Do()
		if err != nil {
			return fmt.Errorf("failed to upload file: %w", err)
		}
		log.Printf("Uploaded file: %s (%d bytes)", remotePath, fileInfo.Size())
	}

	return nil
}

// findFolder finds a folder by name in the given parent
func (c *Client) findFolder(ctx context.Context, name, parentID string) (string, error) {
	query := fmt.Sprintf("name='%s' and mimeType='application/vnd.google-apps.folder' and '%s' in parents and trashed=false", name, parentID)

	files, err := c.service.Files.List().
		Q(query).
		Context(ctx).
		Do()
	if err != nil {
		return "", err
	}

	if len(files.Files) > 0 {
		return files.Files[0].Id, nil
	}

	return "", nil
}

// findFile finds a file by name in the given parent
func (c *Client) findFile(ctx context.Context, name, parentID string) (string, error) {
	query := fmt.Sprintf("name='%s' and '%s' in parents and trashed=false", name, parentID)

	files, err := c.service.Files.List().
		Q(query).
		Context(ctx).
		Do()
	if err != nil {
		return "", err
	}

	if len(files.Files) > 0 {
		return files.Files[0].Id, nil
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

	parentID := c.config.FolderID
	if parentID == "" {
		parentID = "root" // Google Drive root
	}

	for _, part := range parts {
		if part == "" {
			continue
		}

		folderID, err := c.findFolder(ctx, part, parentID)
		if err != nil {
			return "", err
		}

		if folderID == "" {
			return "", fmt.Errorf("folder not found: %s", part)
		}

		parentID = folderID
	}

	return parentID, nil
}
