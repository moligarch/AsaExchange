package messages

import "AsaExchange/internal/core/ports"

// Builder helps construct complex SendMessageParams.
type Builder struct {
	params ports.SendMessageParams
}

// NewBuilder creates a new message builder.
func NewBuilder(chatID int64) *Builder {
	return &Builder{
		params: ports.SendMessageParams{
			ChatID:    chatID,
			ParseMode: "MarkdownV2", // Default to Markdown
		},
	}
}

// WithText sets the message text.
func (b *Builder) WithText(text string) *Builder {
	b.params.Text = text
	return b
}

// WithParseMode overrides the default parse mode.
func (b *Builder) WithParseMode(mode string) *Builder {
	b.params.ParseMode = mode
	return b
}

// WithRemoveKeyboard adds a flag to remove the reply keyboard.
func (b *Builder) WithRemoveKeyboard() *Builder {
	b.params.RemoveKeyboard = true
	b.params.ReplyMarkup = nil // Ensure no other markup is set
	return b
}

// WithContactButton adds a "Share Contact" reply keyboard.
func (b *Builder) WithContactButton(text string) *Builder {
	b.params.RemoveKeyboard = false
	b.params.ReplyMarkup = &ports.ReplyMarkup{
		IsInline: false,
		Buttons: [][]ports.Button{
			{
				{Text: text, RequestContact: true},
			},
		},
	}
	return b
}

// WithInlineButtons adds a set of inline buttons.
func (b *Builder) WithInlineButtons(buttons [][]ports.Button) *Builder {
	b.params.RemoveKeyboard = false
	b.params.ReplyMarkup = &ports.ReplyMarkup{
		IsInline: true,
		Buttons:  buttons,
	}
	return b
}

// WithReplyButtons creates a grid of reply buttons.
// It takes a flat list of button texts and arranges them into rows.
func (b *Builder) WithReplyButtons(buttonTexts []string, columns int) *Builder {
	var rows [][]ports.Button
	var row []ports.Button

	for i, text := range buttonTexts {
		row = append(row, ports.Button{Text: text})
		
		// If we've reached the column limit, or it's the last button
		if (i+1)%columns == 0 || i == len(buttonTexts)-1 {
			rows = append(rows, row)
			row = []ports.Button{} // Start a new row
		}
	}

	b.params.RemoveKeyboard = false
	b.params.ReplyMarkup = &ports.ReplyMarkup{
		IsInline: false,
		Buttons:  rows,
	}
	return b
}

// Build returns the final SendMessageParams struct.
func (b *Builder) Build() ports.SendMessageParams {
	return b.params
}