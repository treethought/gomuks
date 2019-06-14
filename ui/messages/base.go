// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2019 Tulir Asokan
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package messages

import (
	"encoding/json"
	"fmt"
	"time"

	"maunium.net/go/gomuks/config"
	"maunium.net/go/mautrix"
	"maunium.net/go/mauview"
	"maunium.net/go/tcell"

	"maunium.net/go/gomuks/interface"
	"maunium.net/go/gomuks/ui/widget"
)

type MessageRenderer interface {
	Draw(screen mauview.Screen)
	NotificationContent() string
	PlainText() string
	CalculateBuffer(prefs config.UserPreferences, width int, msg *UIMessage)
	RegisterMatrix(matrix ifc.MatrixContainer)
	Height() int
	Clone() MessageRenderer
	String() string
}

type UIMessage struct {
	EventID            string
	TxnID              string
	Relation           mautrix.RelatesTo
	Type               mautrix.MessageType
	SenderID           string
	SenderName         string
	DefaultSenderColor tcell.Color
	Timestamp          time.Time
	State              mautrix.OutgoingEventState
	IsHighlight        bool
	IsService          bool
	Source             json.RawMessage
	ReplyTo            *UIMessage
	Renderer           MessageRenderer
}

const DateFormat = "January _2, 2006"
const TimeFormat = "15:04:05"

func newUIMessage(event *mautrix.Event, displayname string, renderer MessageRenderer) *UIMessage {
	msgtype := event.Content.MsgType
	if len(msgtype) == 0 {
		msgtype = mautrix.MessageType(event.Type.String())
	}

	return &UIMessage{
		SenderID:           event.Sender,
		SenderName:         displayname,
		Timestamp:          unixToTime(event.Timestamp),
		DefaultSenderColor: widget.GetHashColor(event.Sender),
		Type:               msgtype,
		EventID:            event.ID,
		TxnID:              event.Unsigned.TransactionID,
		Relation:           *event.Content.GetRelatesTo(),
		State:              event.Unsigned.OutgoingState,
		IsHighlight:        false,
		IsService:          false,
		Source:             event.Content.VeryRaw,
		Renderer:           renderer,
	}
}

func unixToTime(unix int64) time.Time {
	timestamp := time.Now()
	if unix != 0 {
		timestamp = time.Unix(unix/1000, unix%1000*1000)
	}
	return timestamp
}

// Sender gets the string that should be displayed as the sender of this message.
//
// If the message is being sent, the sender is "Sending...".
// If sending has failed, the sender is "Error".
// If the message is an emote, the sender is blank.
// In any other case, the sender is the display name of the user who sent the message.
func (msg *UIMessage) Sender() string {
	switch msg.State {
	case mautrix.EventStateLocalEcho:
		return "Sending..."
	case mautrix.EventStateSendFail:
		return "Error"
	}
	switch msg.Type {
	case "m.emote":
		// Emotes don't show a separate sender, it's included in the buffer.
		return ""
	default:
		return msg.SenderName
	}
}

func (msg *UIMessage) NotificationSenderName() string {
	return msg.SenderName
}

func (msg *UIMessage) NotificationContent() string {
	return msg.Renderer.NotificationContent()
}

func (msg *UIMessage) getStateSpecificColor() tcell.Color {
	switch msg.State {
	case mautrix.EventStateLocalEcho:
		return tcell.ColorGray
	case mautrix.EventStateSendFail:
		return tcell.ColorRed
	case mautrix.EventStateDefault:
		fallthrough
	default:
		return tcell.ColorDefault
	}
}

// SenderColor returns the color the name of the sender should be shown in.
//
// If the message is being sent, the color is gray.
// If sending has failed, the color is red.
//
// In any other case, the color is whatever is specified in the Message struct.
// Usually that means it is the hash-based color of the sender (see ui/widget/color.go)
func (msg *UIMessage) SenderColor() tcell.Color {
	stateColor := msg.getStateSpecificColor()
	switch {
	case stateColor != tcell.ColorDefault:
		return stateColor
	case msg.Type == "m.room.member":
		return widget.GetHashColor(msg.SenderName)
	case msg.IsService:
		return tcell.ColorGray
	default:
		return msg.DefaultSenderColor
	}
}

// TextColor returns the color the actual content of the message should be shown in.
func (msg *UIMessage) TextColor() tcell.Color {
	stateColor := msg.getStateSpecificColor()
	switch {
	case stateColor != tcell.ColorDefault:
		return stateColor
	case msg.IsService, msg.Type == "m.notice":
		return tcell.ColorGray
	case msg.IsHighlight:
		return tcell.ColorYellow
	case msg.Type == "m.room.member":
		return tcell.ColorGreen
	default:
		return tcell.ColorDefault
	}
}

// TimestampColor returns the color the timestamp should be shown in.
//
// As with SenderColor(), messages being sent and messages that failed to be sent are
// gray and red respectively.
//
// However, other messages are the default color instead of a color stored in the struct.
func (msg *UIMessage) TimestampColor() tcell.Color {
	if msg.IsService {
		return tcell.ColorGray
	}
	return msg.getStateSpecificColor()
}

func (msg *UIMessage) ReplyHeight() int {
	if msg.ReplyTo != nil {
		return 1 + msg.ReplyTo.Height()
	}
	return 0
}

// Height returns the number of rows in the computed buffer (see Buffer()).
func (msg *UIMessage) Height() int {
	return msg.ReplyHeight() + msg.Renderer.Height()
}

func (msg *UIMessage) Time() time.Time {
	return msg.Timestamp
}

// FormatTime returns the formatted time when the message was sent.
func (msg *UIMessage) FormatTime() string {
	return msg.Timestamp.Format(TimeFormat)
}

// FormatDate returns the formatted date when the message was sent.
func (msg *UIMessage) FormatDate() string {
	return msg.Timestamp.Format(DateFormat)
}

func (msg *UIMessage) SameDate(message *UIMessage) bool {
	year1, month1, day1 := msg.Timestamp.Date()
	year2, month2, day2 := message.Timestamp.Date()
	return day1 == day2 && month1 == month2 && year1 == year2
}

func (msg *UIMessage) ID() string {
	if len(msg.EventID) == 0 {
		return msg.TxnID
	}
	return msg.EventID
}

func (msg *UIMessage) SetID(id string) {
	msg.EventID = id
}

func (msg *UIMessage) SetIsHighlight(isHighlight bool) {
	// TODO Textmessage cache needs to be cleared
	msg.IsHighlight = isHighlight
}

func (msg *UIMessage) Draw(screen mauview.Screen) {
	screen = msg.DrawReply(screen)
	msg.Renderer.Draw(screen)
}

func (msg *UIMessage) Clone() *UIMessage {
	clone := *msg
	clone.Renderer = clone.Renderer.Clone()
	return &clone
}

func (msg *UIMessage) CalculateReplyBuffer(preferences config.UserPreferences, width int) {
	if msg.ReplyTo == nil {
		return
	}
	msg.ReplyTo.CalculateBuffer(preferences, width-1)
}

func (msg *UIMessage) CalculateBuffer(preferences config.UserPreferences, width int) {
	msg.Renderer.CalculateBuffer(preferences, width-1, msg)
}

func (msg *UIMessage) DrawReply(screen mauview.Screen) mauview.Screen {
	if msg.ReplyTo == nil {
		return screen
	}
	width, height := screen.Size()
	replyHeight := msg.ReplyTo.Height()
	widget.WriteLineSimpleColor(screen, "In reply to", 1, 0, tcell.ColorGreen)
	widget.WriteLineSimpleColor(screen, msg.ReplyTo.SenderName, 13, 0, msg.ReplyTo.SenderColor())
	for y := 0; y < 1+replyHeight; y++ {
		screen.SetCell(0, y, tcell.StyleDefault, '▊')
	}
	replyScreen := mauview.NewProxyScreen(screen, 1, 1, width-1, replyHeight)
	msg.ReplyTo.Draw(replyScreen)
	return mauview.NewProxyScreen(screen, 0, replyHeight+1, width, height-replyHeight-1)
}

func (msg *UIMessage) String() string {
	return fmt.Sprintf(`&messages.UIMessage{
    ID="%s", TxnID="%s",
    Type="%s", Timestamp=%s,
    Sender={ID="%s", Name="%s", Color=#%X},
    IsService=%t, IsHighlight=%t,
    Renderer=%s,
}`,
		msg.EventID, msg.TxnID,
		msg.Type, msg.Timestamp.String(),
		msg.SenderID, msg.SenderName, msg.DefaultSenderColor.Hex(),
		msg.IsService, msg.IsHighlight, msg.Renderer.String(),
	)
}

func (msg *UIMessage) PlainText() string {
	return msg.Renderer.PlainText()
}
