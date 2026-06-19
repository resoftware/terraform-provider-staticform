package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path/filepath"
	"strings"
)

// UploadedAttachment is the result of uploading an email attachment.
type UploadedAttachment struct {
	S3Key       string `json:"s3Key"`
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	FileSize    int64  `json:"fileSize"`
}

// UploadEmailAttachment uploads a file to a form's email-attachment storage and
// returns its S3 key and metadata. Uses multipart/form-data (field "file").
func (c *Client) UploadEmailAttachment(ctx context.Context, formID, fileName string, content []byte) (*UploadedAttachment, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	ct := mime.TypeByExtension(filepath.Ext(fileName))
	if ct == "" {
		ct = "application/octet-stream"
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, fileName))
	h.Set("Content-Type", ct)
	part, err := mw.CreatePart(h)
	if err != nil {
		return nil, fmt.Errorf("building upload part: %w", err)
	}
	if _, err := part.Write(content); err != nil {
		return nil, fmt.Errorf("writing upload part: %w", err)
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart writer: %w", err)
	}

	url := c.baseURL + "/api/v1/forms/" + formID + "/email-attachments"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseUploadError(resp.StatusCode, raw)
	}

	var out UploadedAttachment
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &out, nil
}

func parseUploadError(status int, raw []byte) (*UploadedAttachment, error) {
	// The upload endpoint returns a bare string for BadRequest; fall back to the
	// shared RFC 7807 parser for everything else.
	if status == http.StatusBadRequest {
		msg := strings.Trim(string(raw), `"`)
		return nil, &APIError{StatusCode: status, Detail: msg, Raw: string(raw)}
	}
	return nil, parseAPIError(status, raw)
}

// DeleteEmailAttachment deletes a previously uploaded attachment by S3 key.
func (c *Client) DeleteEmailAttachment(ctx context.Context, formID, s3Key string) error {
	body := struct {
		S3Key string `json:"s3Key"`
	}{s3Key}
	return c.do(ctx, http.MethodDelete, "/api/v1/forms/"+formID+"/email-attachments", body, nil)
}
