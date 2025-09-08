package sync

// RemoteFileInfo represents information about a file in cloud storage
type RemoteFileInfo struct {
	Path     string
	Size     int64
	MD5Hash  string
	Modified string
}
