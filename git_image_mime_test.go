package main

import "testing"

// getImageMimeType itself is case-sensitive: it compares the extension against
// lowercase literals. Its only caller, GetSourceFileDetail in
// git_project_browser.go, normalizes the input first with
// strings.ToLower(filepath.Ext(filePath)) before calling it, so production
// never passes an uppercase suffix. The uppercase rows below therefore assert
// the function's real (case-sensitive) contract: anything outside the exact
// lowercase set falls through to the octet-stream default.
func TestGetImageMimeType(t *testing.T) {
	tests := []struct {
		name string
		ext  string
		want string
	}{
		{name: "png", ext: ".png", want: "image/png"},
		{name: "jpg", ext: ".jpg", want: "image/jpeg"},
		{name: "jpeg", ext: ".jpeg", want: "image/jpeg"},
		{name: "gif", ext: ".gif", want: "image/gif"},
		{name: "webp", ext: ".webp", want: "image/webp"},
		{name: "svg", ext: ".svg", want: "image/svg+xml"},
		{name: "ico", ext: ".ico", want: "image/x-icon"},
		{name: "bmp", ext: ".bmp", want: "image/bmp"},
		{name: "avif", ext: ".avif", want: "image/avif"},
		{name: "tif", ext: ".tif", want: "image/tiff"},
		{name: "tiff", ext: ".tiff", want: "image/tiff"},
		{name: "unknown extension", ext: ".heic", want: "application/octet-stream"},
		{name: "text extension", ext: ".txt", want: "application/octet-stream"},
		{name: "empty string", ext: "", want: "application/octet-stream"},
		{name: "dot only", ext: ".", want: "application/octet-stream"},
		{name: "extension without dot", ext: "png", want: "application/octet-stream"},
		{name: "full file name is not an extension", ext: "logo.png", want: "application/octet-stream"},
		{name: "uppercase png falls through", ext: ".PNG", want: "application/octet-stream"},
		{name: "mixed case jpeg falls through", ext: ".Jpeg", want: "application/octet-stream"},
		{name: "trailing space falls through", ext: ".png ", want: "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getImageMimeType(tt.ext); got != tt.want {
				t.Fatalf("getImageMimeType(%q) = %q, want %q", tt.ext, got, tt.want)
			}
		})
	}
}

// The image detector and the MIME mapper must stay in lockstep: every extension
// accepted by isImageExtension has to resolve to a real image/* MIME type,
// otherwise the detail view would embed a data:application/octet-stream URL for
// a file it just classified as an image.
func TestImageMimeConsistencyWithImageExtension(t *testing.T) {
	exts := []string{
		".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico", ".bmp",
		".avif", ".tif", ".tiff",
	}
	for _, ext := range exts {
		if !isImageExtension(ext) {
			t.Fatalf("isImageExtension(%q) = false, expected the extension to be treated as an image", ext)
		}
		got := getImageMimeType(ext)
		if got == "application/octet-stream" {
			t.Fatalf("getImageMimeType(%q) returned the default MIME type for an image extension", ext)
		}
		if len(got) < len("image/") || got[:len("image/")] != "image/" {
			t.Fatalf("getImageMimeType(%q) = %q, want an image/* MIME type", ext, got)
		}
	}
}
