package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strings"
	"time"
)

const (
	pluginFrontendEntrypoint      = "index.html"
	pluginFrontendManifestMaxSize = 1 << 20
)

type pluginFrontendSummary struct {
	URL        string `json:"url"`
	Entrypoint string `json:"entrypoint"`
	Assets     int    `json:"assets"`
}

type stagedPluginFrontend struct {
	PluginID   string                `json:"plugin_id"`
	Entrypoint string                `json:"entrypoint"`
	Assets     []PluginFrontendAsset `json:"assets"`
}

func putPluginFrontendAssets(ctx context.Context, blobs BlobStore, pluginID string, files []pluginFrontendFile) (stagedPluginFrontend, error) {
	return putPluginFrontendAssetsReusing(ctx, blobs, pluginID, files, nil)
}

func putPluginFrontendAssetsReusing(
	ctx context.Context,
	blobs BlobStore,
	pluginID string,
	files []pluginFrontendFile,
	previous []PluginFrontendAsset,
) (stagedPluginFrontend, error) {
	staged := stagedPluginFrontend{
		PluginID:   pluginID,
		Entrypoint: pluginFrontendEntrypoint,
	}
	if len(files) == 0 {
		return staged, nil
	}
	if blobs == nil {
		return stagedPluginFrontend{}, fmt.Errorf("blob store is not configured")
	}

	reusableAssets := make(map[string]PluginFrontendAsset, len(previous))
	for _, asset := range previous {
		if asset.Path == "" || asset.Checksum == "" || asset.Key == "" {
			continue
		}
		reusableAssets[frontendAssetReuseKey(asset.Path, asset.Checksum)] = asset
	}

	prefix := fmt.Sprintf(
		"plugin-frontends/%s/%d",
		safeBlobPathSegment(pluginID),
		time.Now().UnixNano(),
	)
	for _, file := range files {
		checksum := frontendAssetChecksum(file.Data)
		if reusable, ok := reusableAssets[frontendAssetReuseKey(file.Path, checksum)]; ok {
			reusable.ContentType = file.ContentType
			reusable.Size = file.Size
			staged.Assets = append(staged.Assets, reusable)
			continue
		}

		assetKey := prefix + "/" + file.Path
		if err := blobs.Put(ctx, assetKey, bytes.NewReader(file.Data), file.Size); err != nil {
			deleteFrontendAssetsBestEffort(ctx, blobs, staged.Assets)
			return stagedPluginFrontend{}, fmt.Errorf("save frontend asset %q: %w", file.Path, err)
		}
		staged.Assets = append(staged.Assets, PluginFrontendAsset{
			Path:        file.Path,
			Key:         assetKey,
			ContentType: file.ContentType,
			Checksum:    checksum,
			Size:        file.Size,
		})
	}
	return staged, nil
}

func saveStagedPluginFrontendManifest(ctx context.Context, blobs BlobStore, wasmKey string, staged stagedPluginFrontend) error {
	raw, err := json.Marshal(staged)
	if err != nil {
		return fmt.Errorf("marshal frontend manifest: %w", err)
	}
	if err := blobs.Put(ctx, pluginFrontendManifestKey(wasmKey), bytes.NewReader(raw), int64(len(raw))); err != nil {
		return fmt.Errorf("save frontend manifest: %w", err)
	}
	return nil
}

func readStagedPluginFrontendManifest(ctx context.Context, blobs BlobStore, wasmKey string) (stagedPluginFrontend, bool, error) {
	if blobs == nil {
		return stagedPluginFrontend{}, false, nil
	}

	manifestKey := pluginFrontendManifestKey(wasmKey)
	exists, err := blobs.Exists(ctx, manifestKey)
	if err != nil {
		return stagedPluginFrontend{}, false, fmt.Errorf("check frontend manifest %q: %w", manifestKey, err)
	}
	if !exists {
		return stagedPluginFrontend{}, false, nil
	}

	rc, err := blobs.Get(ctx, manifestKey)
	if err != nil {
		return stagedPluginFrontend{}, false, fmt.Errorf("get frontend manifest %q: %w", manifestKey, err)
	}
	defer rc.Close()

	raw, err := io.ReadAll(io.LimitReader(rc, pluginFrontendManifestMaxSize+1))
	if err != nil {
		return stagedPluginFrontend{}, false, fmt.Errorf("read frontend manifest %q: %w", manifestKey, err)
	}
	if len(raw) > pluginFrontendManifestMaxSize {
		return stagedPluginFrontend{}, false, fmt.Errorf("frontend manifest %q is too large", manifestKey)
	}

	var staged stagedPluginFrontend
	if err := json.Unmarshal(raw, &staged); err != nil {
		return stagedPluginFrontend{}, false, fmt.Errorf("decode frontend manifest %q: %w", manifestKey, err)
	}
	if staged.Entrypoint == "" {
		staged.Entrypoint = pluginFrontendEntrypoint
	}
	for _, asset := range staged.Assets {
		cleanPath, ok := cleanFrontendAssetPath(asset.Path)
		if !ok || cleanPath == "" || cleanPath != asset.Path || asset.Key == "" {
			return stagedPluginFrontend{}, false, fmt.Errorf("frontend manifest %q contains invalid asset", manifestKey)
		}
	}
	return staged, true, nil
}

func pluginFrontendManifestKey(wasmKey string) string {
	return wasmKey + ".frontend.json"
}

func pluginFrontendAppURL(pluginID string) string {
	return "/plugins/" + url.PathEscape(pluginID) + "/app/"
}

func pluginFrontendSummaryFromRecord(record PluginFrontendRecord) pluginFrontendSummary {
	entrypoint := record.Entrypoint
	if entrypoint == "" {
		entrypoint = pluginFrontendEntrypoint
	}
	return pluginFrontendSummary{
		URL:        pluginFrontendAppURL(record.PluginID),
		Entrypoint: entrypoint,
		Assets:     len(record.Assets),
	}
}

func pluginFrontendStoreFrom(store PluginStore) (PluginFrontendStore, bool) {
	frontendStore, ok := store.(PluginFrontendStore)
	return frontendStore, ok
}

func deleteFrontendAssetsBestEffort(ctx context.Context, blobs BlobStore, assets []PluginFrontendAsset) {
	if blobs == nil {
		return
	}
	for _, asset := range assets {
		if asset.Key == "" {
			continue
		}
		if err := blobs.Delete(ctx, asset.Key); err != nil {
			slog.Warn("admin: delete plugin frontend asset", "key", asset.Key, "error", err)
		}
	}
}

func deleteUnreferencedFrontendAssetsBestEffort(ctx context.Context, blobs BlobStore, previous, next []PluginFrontendAsset) {
	if len(previous) == 0 {
		return
	}
	nextKeys := make(map[string]struct{}, len(next))
	for _, asset := range next {
		if asset.Key != "" {
			nextKeys[asset.Key] = struct{}{}
		}
	}

	var deleted []PluginFrontendAsset
	for _, asset := range previous {
		if _, stillUsed := nextKeys[asset.Key]; stillUsed {
			continue
		}
		deleted = append(deleted, asset)
	}
	deleteFrontendAssetsBestEffort(ctx, blobs, deleted)
}

func frontendAssetChecksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func frontendAssetReuseKey(assetPath, checksum string) string {
	return assetPath + "\x00" + checksum
}

func safeBlobPathSegment(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "plugin"
	}
	return b.String()
}
