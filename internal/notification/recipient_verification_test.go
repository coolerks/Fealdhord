package notification

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestRecipientVerificationChecksTargetCodeAndRevision(t *testing.T) {
	s, now := newTestNotification(t)
	ctx := t.Context()
	input := ChannelInput{
		Channel: Channel{Name: "Telegram", Platform: "telegram", Enabled: true},
		Address: "1234:bot-token", Targets: []string{"42", "43"},
	}
	channel, err := s.SaveChannel(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	a, b := channel.Recipients[0], channel.Recipients[1]
	codes := map[string]string{}
	sends := 0
	s.transport.client.Transport = sender(func(r *http.Request) (*http.Response, error) {
		var body struct {
			ChatID string `json:"chat_id"`
			Text   string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		code := strings.TrimPrefix(body.Text, "配对成功，校验码：")
		if !verificationDigits.MatchString(code) {
			t.Fatalf("invalid verification message %q", body.Text)
		}
		sends++
		codes[body.ChatID] = code
		return response(200, `{"ok":true}`), nil
	})
	send := func(recipient Recipient, resend bool) error {
		return s.sendRecipientVerification(ctx, channel.ID, channel.Revision, recipient.ID, "", resend)
	}
	confirm := func(recipient Recipient, code string) error {
		return s.confirmRecipientVerification(ctx, channel.ID, channel.Revision, recipient.ID, code)
	}
	if err := send(a, false); err != nil {
		t.Fatal(err)
	}
	if err := send(a, false); err != nil || sends != 1 {
		t.Fatalf("duplicate automatic send: %d, %v", sends, err)
	}
	if err := send(b, false); err != nil {
		t.Fatal(err)
	}
	if err := send(b, true); err == nil {
		t.Fatal("immediate resend was allowed")
	}
	if err := confirm(a, codes[b.Target]); err == nil {
		t.Fatal("accepted the other target's code")
	}
	current, err := s.ListChannels(ctx)
	if err != nil || len(current) != 1 || current[0].Verified {
		t.Fatalf("channel verified before code confirmation: %+v, %v", current, err)
	}
	if err := confirm(a, codes[a.Target]); err != nil {
		t.Fatal(err)
	}
	if err := confirm(a, codes[a.Target]); err == nil {
		t.Fatal("accepted a used code")
	}
	current, err = s.ListChannels(ctx)
	if err != nil || !current[0].Verified || !current[0].Recipients[0].Verified || current[0].Recipients[1].Verified {
		t.Fatalf("recipient flags %+v, %v", current, err)
	}
	for range verificationAttempts {
		if err := confirm(b, codes[a.Target]); err == nil {
			t.Fatal("accepted an incorrect code")
		}
	}
	if err := confirm(b, codes[b.Target]); err == nil {
		t.Fatal("accepted a code after exhausting its attempts")
	}

	*now = now.Add(verificationLifetime)
	if err := confirm(b, codes[b.Target]); err == nil {
		t.Fatal("accepted an expired code")
	}
	if err := send(b, true); err != nil {
		t.Fatal(err)
	}
	input.Channel = channel
	input.Name = "更新后的渠道"
	if _, err := s.SaveChannel(ctx, input); err != nil {
		t.Fatal(err)
	}
	if err := confirm(b, codes[b.Target]); err == nil {
		t.Fatal("accepted a code issued for an earlier revision")
	}
}

func TestQQVerificationRepliesToPairingMessage(t *testing.T) {
	s, now := newTestNotification(t)
	ctx := t.Context()
	const target = "c2c:recipient"
	channel, err := s.SaveChannel(ctx, ChannelInput{
		Channel: Channel{Name: "QQ", Platform: "qq", Enabled: true},
		Address: "app", Signing: "secret", Targets: []string{target},
	})
	if err != nil {
		t.Fatal(err)
	}
	s.pairing.status.ID = "match"
	s.pairing.qqAppID = "app"
	s.pairing.qqReplies = map[string]qqPairingReply{target: {id: "inbound-message-id", received: *now}}
	var code string
	s.transport.client.Transport = sender(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/app/getAppAccessToken":
			return response(200, `{"access_token":"expected"}`), nil
		case "/v2/users/recipient/messages":
			var body struct {
				ID       string `json:"msg_id"`
				Sequence int    `json:"msg_seq"`
				Content  string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID != "inbound-message-id" || body.Sequence != 1 {
				t.Fatalf("QQ message must reply to the matched event: %+v, %v", body, err)
			}
			code = strings.TrimPrefix(body.Content, "配对成功，校验码：")
			return response(200, `{"id":"reply-id"}`), nil
		default:
			t.Fatalf("unexpected request %s", r.URL.Path)
			return response(500, "{}"), nil
		}
	})
	id := channel.Recipients[0].ID
	if err := s.sendRecipientVerification(ctx, channel.ID, channel.Revision, id, "match", false); err != nil {
		t.Fatal(err)
	}
	if err := s.confirmRecipientVerification(ctx, channel.ID, channel.Revision, id, code); err != nil {
		t.Fatal(err)
	}
}
