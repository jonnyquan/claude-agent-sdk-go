package shared

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestImageBlock(t *testing.T) {
	// Create sample base64 image data (a simple 1x1 pixel PNG)
	pngBytes := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde, 0x00, 0x00, 0x00,
		0x09, 0x70, 0x48, 0x59, 0x73, 0x00, 0x00, 0x0b, 0x13, 0x00, 0x00, 0x0b,
		0x13, 0x01, 0x00, 0x9a, 0x9c, 0x18, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44,
		0x41, 0x54, 0x78, 0x9c, 0x63, 0x60, 0x60, 0x60, 0x00, 0x00, 0x00, 0x04,
		0x00, 0x01, 0x5d, 0x55, 0x21, 0x1c, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45,
		0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
	pngData := base64.StdEncoding.EncodeToString(pngBytes)

	t.Run("ImageBlock creation", func(t *testing.T) {
		block := &ImageBlock{
			MessageType: ContentBlockTypeImage,
			Data:        pngData,
			MimeType:    "image/png",
		}

		if block.BlockType() != ContentBlockTypeImage {
			t.Errorf("Expected block type %s, got %s", ContentBlockTypeImage, block.BlockType())
		}

		if block.Data != pngData {
			t.Error("Image data mismatch")
		}

		if block.MimeType != "image/png" {
			t.Errorf("Expected mime type image/png, got %s", block.MimeType)
		}
	})

	t.Run("ImageBlock JSON marshaling", func(t *testing.T) {
		block := &ImageBlock{
			MessageType: ContentBlockTypeImage,
			Data:        pngData,
			MimeType:    "image/png",
		}

		data, err := json.Marshal(block)
		if err != nil {
			t.Fatalf("Failed to marshal ImageBlock: %v", err)
		}

		var unmarshaled ImageBlock
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("Failed to unmarshal ImageBlock: %v", err)
		}

		if unmarshaled.MessageType != ContentBlockTypeImage {
			t.Errorf("Expected type %s, got %s", ContentBlockTypeImage, unmarshaled.MessageType)
		}

		if unmarshaled.Data != pngData {
			t.Error("Image data mismatch after unmarshaling")
		}

		if unmarshaled.MimeType != "image/png" {
			t.Errorf("Expected mime type image/png, got %s", unmarshaled.MimeType)
		}
	})

	t.Run("ImageBlock with different mime types", func(t *testing.T) {
		testCases := []struct {
			name     string
			mimeType string
		}{
			{"PNG", "image/png"},
			{"JPEG", "image/jpeg"},
			{"GIF", "image/gif"},
			{"WebP", "image/webp"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				block := &ImageBlock{
					MessageType: ContentBlockTypeImage,
					Data:        pngData,
					MimeType:    tc.mimeType,
				}

				if block.MimeType != tc.mimeType {
					t.Errorf("Expected mime type %s, got %s", tc.mimeType, block.MimeType)
				}
			})
		}
	})

	t.Run("ImageBlock in AssistantMessage content", func(t *testing.T) {
		textBlock := &TextBlock{
			MessageType: ContentBlockTypeText,
			Text:        "Here is a chart:",
		}

		imageBlock := &ImageBlock{
			MessageType: ContentBlockTypeImage,
			Data:        pngData,
			MimeType:    "image/png",
		}

		msg := &AssistantMessage{
			MessageType: MessageTypeAssistant,
			Content:     []ContentBlock{textBlock, imageBlock},
			Model:       "claude-sonnet-4-5",
		}

		if len(msg.Content) != 2 {
			t.Errorf("Expected 2 content blocks, got %d", len(msg.Content))
		}

		// Verify first block is text
		if msg.Content[0].BlockType() != ContentBlockTypeText {
			t.Errorf("Expected first block to be text, got %s", msg.Content[0].BlockType())
		}

		// Verify second block is image
		if msg.Content[1].BlockType() != ContentBlockTypeImage {
			t.Errorf("Expected second block to be image, got %s", msg.Content[1].BlockType())
		}

		// Type assert and verify image data
		if img, ok := msg.Content[1].(*ImageBlock); ok {
			if img.Data != pngData {
				t.Error("Image data mismatch in message content")
			}
			if img.MimeType != "image/png" {
				t.Errorf("Expected mime type image/png, got %s", img.MimeType)
			}
		} else {
			t.Error("Failed to type assert ImageBlock")
		}
	})
}

func TestImageBlockConstant(t *testing.T) {
	if ContentBlockTypeImage != "image" {
		t.Errorf("Expected ContentBlockTypeImage to be 'image', got '%s'", ContentBlockTypeImage)
	}
}
