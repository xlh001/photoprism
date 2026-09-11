package event

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/photoprism/photoprism/pkg/i18n"
)

func TestSuccessMsg(t *testing.T) {
	t.Run("WithParams", func(t *testing.T) {
		s := Subscribe("notify.success")
		SuccessMsg(i18n.MsgAlbumDeleted, "Holiday")
		msg := <-s.Receiver
		assert.Equal(t, "notify.success", msg.Name)
		assert.Equal(t, "Album Holiday deleted", msg.Fields["message"])
		assert.Equal(t, "Album %s deleted", msg.Fields["messageId"])
		assert.Equal(t, []any{"Holiday"}, msg.Fields["messageParams"])
		Unsubscribe(s)
	})
	t.Run("WithoutParams", func(t *testing.T) {
		s := Subscribe("notify.success")
		SuccessMsg(i18n.MsgAlbumCreated)
		msg := <-s.Receiver
		assert.Equal(t, "notify.success", msg.Name)
		assert.Equal(t, "Album created", msg.Fields["message"])
		assert.Equal(t, "Album created", msg.Fields["messageId"])
		Unsubscribe(s)
	})
}

func TestErrorMsg(t *testing.T) {
	s := Subscribe("notify.error")
	ErrorMsg(i18n.ErrAlreadyExists, "A cat")
	msg := <-s.Receiver
	assert.Equal(t, "notify.error", msg.Name)
	assert.Equal(t, "A cat already exists", msg.Fields["message"])
	assert.Equal(t, "%s already exists", msg.Fields["messageId"])
	assert.Equal(t, []any{"A cat"}, msg.Fields["messageParams"])
	Unsubscribe(s)
}

func TestInfoMsg(t *testing.T) {
	s := Subscribe("notify.info")
	InfoMsg(i18n.MsgIndexingFiles, "/photos")
	msg := <-s.Receiver
	assert.Equal(t, "notify.info", msg.Name)
	assert.Equal(t, "Indexing files in /photos", msg.Fields["message"])
	assert.Equal(t, "Indexing files in %s", msg.Fields["messageId"])
	assert.Equal(t, []any{"/photos"}, msg.Fields["messageParams"])
	Unsubscribe(s)
}

func TestWarnMsg(t *testing.T) {
	s := Subscribe("notify.warning")
	WarnMsg(i18n.ErrBusy)
	msg := <-s.Receiver
	assert.Equal(t, "notify.warning", msg.Name)
	assert.Equal(t, "Busy, please try again later", msg.Fields["message"])
	assert.Equal(t, "Busy, please try again later", msg.Fields["messageId"])
	Unsubscribe(s)
}

func TestPublishCompleted(t *testing.T) {
	// Each case subscribes before publishing, because the hub delivers to current subscribers.
	collect := func(topics []string, want int, fn func()) []Message {
		sub := Subscribe(topics...)
		defer Unsubscribe(sub)

		fn()

		var got []Message

		for range want {
			select {
			case msg := <-sub.Receiver:
				got = append(got, msg)
			case <-time.After(2 * time.Second):
				return got
			}
		}

		return got
	}

	t.Run("Fields", func(t *testing.T) {
		got := collect([]string{"upload.completed"}, 1, func() {
			PublishCompleted([]string{"upload.completed"}, "urqjjgt71ap1w2gr", "", 7)
		})
		require.Len(t, got, 1)
		assert.Equal(t, "urqjjgt71ap1w2gr", got[0].Fields["uid"])
		assert.Equal(t, 7, got[0].Fields["seconds"])
		assert.NotContains(t, got[0].Fields, "action")
		// No path: nothing reads one, and topic routing reaches every permitted session
		// rather than the one that acted.
		assert.NotContains(t, got[0].Fields, "path")
	})
	t.Run("Action", func(t *testing.T) {
		got := collect([]string{"index.completed"}, 1, func() {
			PublishCompleted([]string{"index.completed"}, "urqjjgt71ap1w2gr", "index", 3)
		})
		require.Len(t, got, 1)
		assert.Equal(t, "index", got[0].Fields["action"])
	})
	t.Run("EveryTopicSamePayload", func(t *testing.T) {
		topics := []string{"import.completed", "index.completed", "upload.completed"}
		got := collect(topics, 3, func() {
			PublishCompleted(topics, "urqjjgt71ap1w2gr", "import", 1)
		})
		require.Len(t, got, 3)

		seen := map[string]bool{}

		for _, msg := range got {
			seen[msg.Name] = true
			// Asserted per message, not once: a payload that differs per topic would
			// otherwise pass while only the first carried the fields.
			assert.Equal(t, "urqjjgt71ap1w2gr", msg.Fields["uid"], msg.Name)
			assert.Equal(t, "import", msg.Fields["action"], msg.Name)
			assert.Equal(t, 1, msg.Fields["seconds"], msg.Name)
		}

		assert.Len(t, seen, 3)
	})
}
