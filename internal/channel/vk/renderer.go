package vk

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf16"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/model"

	vkobject "github.com/SevereCloud/vksdk/v3/object"
)

const (
	vkMaxMessageLength = 9000
	vkMaxButtonLabel   = 40
	vkPayloadKey       = "sb"
	maxKeyboardButtons = 10
)

type buttonPayload struct {
	Value string `json:"sb"`
}

type RenderedMessage struct {
	Text       string
	FormatData *vkFormatData
	Keyboard   *vkobject.MessagesKeyboard
	ImageURLs  []string
	FileRefs   []model.FileRef
}

type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

type vkFormatData struct {
	Version int            `json:"version"`
	Items   []vkFormatItem `json:"items,omitempty"`
}

func (d vkFormatData) ToJSON() string {
	raw, _ := json.Marshal(d)
	return string(raw)
}

type vkFormatItem struct {
	Type   string `json:"type"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
	URL    string `json:"url,omitempty"`
}

type renderedFragment struct {
	Text        string
	FormatItems []vkFormatItem
}

func (r *Renderer) Render(msg model.Message) RenderedMessage {
	fragments := make([]renderedFragment, 0, len(msg.Blocks))
	imageURLs := make([]string, 0)
	fileRefs := make([]model.FileRef, 0)
	var options *model.OptionsBlock

	for _, block := range msg.Blocks {
		switch b := block.(type) {
		case model.TextBlock:
			fragments = append(fragments, renderTextFragment(b))
		case model.MentionBlock:
			fragments = append(fragments, renderMentionFragment(b))
		case model.LinkBlock:
			fragments = append(fragments, renderLinkFragment(b))
		case model.ImageBlock:
			imageURLs = append(imageURLs, b.URL)
		case model.OptionsBlock:
			if b.Prompt != "" {
				fragments = append(fragments, renderedFragment{Text: b.Prompt})
			}

			normalized, extraText := normalizeVKOptions(&b)
			options = normalized
			for _, line := range extraText {
				fragments = append(fragments, renderedFragment{Text: line})
			}
		case model.FileBlock:
			fileRefs = append(fileRefs, b.FileRef)
			if b.Caption != "" {
				fragments = append(fragments, renderTextFragment(model.TextBlock{Text: b.Caption}))
			}
		}
	}

	text, formatData := assembleFragments(fragments)
	if runeCount(text) > vkMaxMessageLength {
		text, formatData = truncateRenderedText(text, formatData, vkMaxMessageLength)
	}

	var formatDataPtr *vkFormatData
	if len(formatData.Items) > 0 {
		formatCopy := formatData
		formatDataPtr = &formatCopy
	}

	return RenderedMessage{
		Text:       text,
		FormatData: formatDataPtr,
		Keyboard:   buildKeyboard(options),
		ImageURLs:  imageURLs,
		FileRefs:   fileRefs,
	}
}

func buildKeyboard(options *model.OptionsBlock) *vkobject.MessagesKeyboard {
	if options == nil || len(options.Options) == 0 {
		return nil
	}

	keyboard := vkobject.NewMessagesKeyboardInline()
	limit := len(options.Options)
	if limit > maxKeyboardButtons {
		limit = maxKeyboardButtons
	}

	for _, opt := range options.Options[:limit] {
		label := vkButtonLabel(opt)
		if label == "" {
			continue
		}
		// Inline keyboards allow fewer rows than reply keyboards.
		// Two buttons per row keep all ten choices within five rows.
		if len(keyboard.Buttons) == 0 || len(keyboard.Buttons[len(keyboard.Buttons)-1]) >= 2 {
			keyboard.AddRow()
		}
		keyboard.AddCallbackButton(label, buttonPayload{Value: opt.Value}, vkobject.Primary)
	}

	return keyboard
}

func normalizeVKOptions(options *model.OptionsBlock) (*model.OptionsBlock, []string) {
	if options == nil || len(options.Options) == 0 {
		return options, nil
	}

	normalized := *options
	normalized.Options = make([]model.Option, 0, len(options.Options))
	extraText := make([]string, 0, len(options.Options))

	for _, opt := range options.Options {
		normalizedOpt := opt
		label := channel.OptionLabel(opt)

		if commandLabel, ok := shortVKCommandLabel(label); ok {
			normalizedOpt.Label = commandLabel
			extraText = append(extraText, label)
		} else {
			normalizedOpt.Label = vkButtonLabel(opt)
		}

		normalized.Options = append(normalized.Options, normalizedOpt)
	}

	return &normalized, extraText
}

func shortVKCommandLabel(label string) (string, bool) {
	if !strings.HasPrefix(label, "/") {
		return "", false
	}

	head, _, ok := strings.Cut(label, " — ")
	if !ok {
		return "", false
	}
	head = strings.TrimSpace(head)
	if head == "" {
		return "", false
	}
	if runeCount(head) > vkMaxButtonLabel {
		return "", false
	}

	return head, true
}

func vkButtonLabel(opt model.Option) string {
	label := channel.OptionLabel(opt)
	if label == "" {
		return ""
	}

	if runeCount(label) <= vkMaxButtonLabel {
		return label
	}

	// Prefer the concise command/button title without the description suffix.
	if head, _, ok := strings.Cut(label, " — "); ok && runeCount(head) <= vkMaxButtonLabel {
		return head
	}

	if runeCount(opt.Value) > 0 && runeCount(opt.Value) <= vkMaxButtonLabel {
		return opt.Value
	}

	return truncateRunes(label, vkMaxButtonLabel)
}

func truncateRunes(s string, limit int) string {
	if limit <= 0 {
		return ""
	}

	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	if limit == 1 {
		return "…"
	}

	return string(runes[:limit-1]) + "…"
}

func runeCount(s string) int {
	return len([]rune(s))
}

func prefixLines(text, prefix string) string {
	if text == "" {
		return ""
	}

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func renderTextFragment(b model.TextBlock) renderedFragment {
	switch b.Style {
	case model.StyleHeader:
		return renderStyledLinesFragment(b.Text, "bold")
	case model.StyleSubheader:
		return renderStyledLinesFragment(b.Text, "italic")
	case model.StyleQuote:
		return renderedFragment{Text: prefixLines(b.Text, "> ")}
	default:
		return renderedFragment{Text: b.Text}
	}
}

func renderStyledLinesFragment(text, formatType string) renderedFragment {
	fragment := renderedFragment{Text: text}
	if text == "" {
		return fragment
	}

	lines := strings.Split(text, "\n")
	offset := 0
	for i, line := range lines {
		if line != "" {
			fragment.FormatItems = append(fragment.FormatItems, vkFormatItem{
				Type:   formatType,
				Offset: offset,
				Length: utf16Len(line),
			})
		}

		offset += utf16Len(line)
		if i < len(lines)-1 {
			offset++
		}
	}

	return fragment
}

func renderMentionFragment(b model.MentionBlock) renderedFragment {
	username := b.Username
	if username == "" {
		username = "user"
	}

	return renderedFragment{
		Text: fmt.Sprintf("[id%s|%s]", b.UserID, username),
	}
}

func renderLinkFragment(b model.LinkBlock) renderedFragment {
	text := b.Label
	if text == "" {
		text = b.URL
	}

	fragment := renderedFragment{Text: text}
	if text == "" || b.URL == "" {
		return fragment
	}

	fragment.FormatItems = append(fragment.FormatItems, vkFormatItem{
		Type:   "url",
		Offset: 0,
		Length: utf16Len(text),
		URL:    b.URL,
	})
	return fragment
}

func assembleFragments(fragments []renderedFragment) (string, vkFormatData) {
	var text strings.Builder
	formatData := vkFormatData{Version: 1}
	offset := 0

	for i, fragment := range fragments {
		if i > 0 {
			text.WriteByte('\n')
			offset++
		}

		text.WriteString(fragment.Text)
		for _, item := range fragment.FormatItems {
			shifted := item
			shifted.Offset += offset
			formatData.Items = append(formatData.Items, shifted)
		}

		offset += utf16Len(fragment.Text)
	}

	return text.String(), formatData
}

func truncateRenderedText(text string, formatData vkFormatData, limit int) (string, vkFormatData) {
	if limit <= 0 || runeCount(text) <= limit {
		return text, formatData
	}

	runes := []rune(text)
	if limit <= 3 {
		text = string(runes[:limit])
		formatData.Items = clipFormatItems(formatData.Items, utf16Len(text))
		return text, formatData
	}

	prefix := string(runes[:limit-3])
	text = prefix + "..."
	formatData.Items = clipFormatItems(formatData.Items, utf16Len(prefix))
	return text, formatData
}

func clipFormatItems(items []vkFormatItem, maxUnits int) []vkFormatItem {
	clipped := make([]vkFormatItem, 0, len(items))
	for _, item := range items {
		if item.Offset >= maxUnits {
			continue
		}

		end := item.Offset + item.Length
		if end > maxUnits {
			item.Length = maxUnits - item.Offset
		}
		if item.Length <= 0 {
			continue
		}

		clipped = append(clipped, item)
	}

	return clipped
}

func utf16Len(text string) int {
	return len(utf16.Encode([]rune(text)))
}
