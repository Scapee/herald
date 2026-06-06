package workflow

import (
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"

	"github.com/JaimeStill/herald/internal/state"

	taustate "github.com/tailored-agentic-units/orchestrate/state"
)

func InitNode(rt *Runtime) taustate.StateNode {
	return taustate.NewFunctionNode(func(
		ctx context.Context,
		s taustate.State,
	) (taustate.State, error) {
		rt.Logger.InfoContext(ctx, "init node: starting")

		documentID, tempDir, err := extractInitState(s)
		if err != nil {
			return s, fmt.Errorf("init: %w", err)
		}
		rt.Logger.InfoContext(ctx, "init node: state extracted", "document_id", documentID, "temp_dir", tempDir)

		doc, err := rt.Documents.Find(ctx, documentID)
		if err != nil {
			rt.Logger.ErrorContext(ctx, "init node: document lookup failed", "document_id", documentID, "error", err)
			return s, fmt.Errorf("init: %w: %w", ErrDocumentNotFound, err)
		}
		rt.Logger.InfoContext(ctx, "init node: document found", "document_id", documentID, "filename", doc.Filename, "content_type", doc.ContentType, "storage_key", doc.StorageKey)

		handler, err := rt.Formats.Lookup(doc.ContentType)
		if err != nil {
			rt.Logger.ErrorContext(ctx, "init node: format lookup failed", "content_type", doc.ContentType, "error", err)
			return s, fmt.Errorf("init: %w: %w", ErrRenderFailed, err)
		}
		rt.Logger.InfoContext(ctx, "init node: format handler resolved", "format", handler.ID())

		src := &blobSource{
			rt:          rt,
			storageKey:  doc.StorageKey,
			contentType: doc.ContentType,
			filename:    doc.Filename,
		}

		rt.Logger.InfoContext(ctx, "init node: extracting pages from blob")
		pages, err := handler.Extract(ctx, src, tempDir)
		if err != nil {
			rt.Logger.ErrorContext(ctx, "init node: page extraction failed", "error", err)
			return s, fmt.Errorf("init: %w: %w", ErrRenderFailed, err)
		}

		rt.Logger.InfoContext(
			ctx, "init node complete",
			"document_id", documentID,
			"format", handler.ID(),
			"page_count", len(pages),
		)

		s = s.Set(state.KeyClassState, state.ClassificationState{Pages: pages})
		s = s.Set(state.KeyFilename, doc.Filename)
		s = s.Set(state.KeyPageCount, len(pages))

		return s, nil
	})
}

func extractInitState(s taustate.State) (uuid.UUID, string, error) {
	docIDVal, ok := s.Get(state.KeyDocumentID)
	if !ok {
		return uuid.Nil, "", fmt.Errorf("%w: missing %s in state", ErrDocumentNotFound, state.KeyDocumentID)
	}

	documentID, ok := docIDVal.(uuid.UUID)
	if !ok {
		return uuid.Nil, "", fmt.Errorf("%w: %s is not uuid.UUID", ErrDocumentNotFound, state.KeyDocumentID)
	}

	tempDirVal, ok := s.Get(state.KeyTempDir)
	if !ok {
		return uuid.Nil, "", fmt.Errorf("%w: missing %s in state", ErrRenderFailed, state.KeyTempDir)
	}

	tempDir, ok := tempDirVal.(string)
	if !ok {
		return uuid.Nil, "", fmt.Errorf("%w: %s is not string", ErrRenderFailed, state.KeyTempDir)
	}

	return documentID, tempDir, nil
}

type blobSource struct {
	rt          *Runtime
	storageKey  string
	contentType string
	filename    string
}

func (b *blobSource) Open(ctx context.Context) (io.ReadCloser, error) {
	blob, err := b.rt.Storage.Download(ctx, b.storageKey)
	if err != nil {
		return nil, err
	}
	return blob.Body, nil
}

func (b *blobSource) ContentType() string { return b.contentType }
func (b *blobSource) Filename() string    { return b.filename }
