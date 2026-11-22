package notification

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestGatewayPairingTargets(t *testing.T) {
	tests := []struct {
		name, platform, event, data, want string
	}{
		{"QQ private", "qq", "C2C_MESSAGE_CREATE", `{"content":" 配对 ","author":{"user_openid":"direct_012"}}`, "c2c:direct_012"},
		{"QQ group", "qq", "GROUP_AT_MESSAGE_CREATE", `{"content":"<@!123> 配对","group_openid":"group_012","author":{"member_openid":"different"}}`, "group:group_012"},
		{"QQ full group with mention", "qq", "GROUP_MESSAGE_CREATE", `{"content":"配对","group_openid":"group_012","mentions":[{"id":"123"}]}`, "group:group_012"},
		{"QQ full group without mention", "qq", "GROUP_MESSAGE_CREATE", `{"content":"配对","group_openid":"group_012"}`, ""},
		{"QQ guild", "qq", "AT_MESSAGE_CREATE", `{"content":"<@123>配对","channel_id":"42","guild_id":"12"}`, "channel:42"},
		{"QQ unrelated mention", "qq", "AT_MESSAGE_CREATE", `{"content":"<@999> 配对","channel_id":"42"}`, ""},
		{"QQ member without group", "qq", "GROUP_AT_MESSAGE_CREATE", `{"content":"配对","author":{"member_openid":"different"}}`, ""},
		{"QQ ordinary text", "qq", "C2C_MESSAGE_CREATE", `{"content":"怎么配对？","author":{"user_openid":"direct_012"}}`, ""},
		{"Discord DM", "discord", "MESSAGE_CREATE", `{"content":"配对","author":{"id":"42"}}`, "42"},
		{"Discord guild", "discord", "MESSAGE_CREATE", `{"content":"配对","guild_id":"12","author":{"id":"42"}}`, ""},
		{"Discord bot", "discord", "MESSAGE_CREATE", `{"content":"配对","author":{"id":"42","bot":true}}`, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := gatewayPairingTarget(test.platform, "123", pairingGatewayFrame{Event: test.event, Data: json.RawMessage(test.data)})
			if got != test.want {
				t.Fatalf("target = %q, want %q", got, test.want)
			}
		})
	}
}

func TestTelegramPairingIsTemporaryAndRequiresConfirmation(t *testing.T) {
	service, _ := newTestNotification(t)
	api := NewAPI(service, nil)
	requests := 0
	cancelled := make(chan struct{})
	service.transport.client.Transport = sender(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.telegram.org" || !strings.HasSuffix(r.URL.Path, "/getUpdates") {
			t.Errorf("unexpected request %s", r.URL)
			return response(500, "{}"), nil
		}
		var request struct {
			Offset int64 `json:"offset"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		requests++
		switch requests {
		case 1:
			return response(200, `{"ok":true,"result":[{"update_id":1,"message":{"text":"配对","date":4102444800,"chat":{"id":100}}}]}`), nil
		case 2:
			if request.Offset != 2 {
				t.Errorf("offset = %d, want 2", request.Offset)
			}
			return response(200, `{"ok":true,"result":[{"update_id":2,"message":{"text":"配对","date":4102444800,"chat":{"id":42}}},{"update_id":3,"message":{"text":"配对","date":10,"chat":{"id":50}}},{"update_id":4,"message":{"text":"如何配对","date":4102444800,"chat":{"id":51}}}]}`), nil
		default:
			<-r.Context().Done()
			close(cancelled)
			return nil, r.Context().Err()
		}
	})
	input := ChannelInput{Channel: Channel{Platform: "telegram", Name: "Telegram", Enabled: true}, Address: "42:token"}
	pair, err := api.StartRecipientPairing(t.Context(), input, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { api.CancelRecipientPairing(pair.ID) })
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && len(api.RecipientPairingStatus(pair.ID).Targets) == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if got := api.RecipientPairingStatus(pair.ID); got.State != "listening" || !slices.Equal(got.Targets, []string{"42"}) {
		t.Fatalf("unexpected status %+v", got)
	}
	channels, err := api.ListChannels(t.Context())
	if err != nil || len(channels) != 0 {
		t.Fatalf("pairing must not persist a channel: %v, %v", channels, err)
	}
	api.CancelRecipientPairing(pair.ID)
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("cancelling did not end the active long poll")
	}
	if got := api.RecipientPairingStatus(pair.ID); got.State != "expired" || len(got.Targets) != 0 {
		t.Fatalf("cancelled status %+v", got)
	}
	input.Targets = []string{}
	channel, err := api.SaveChannel(t.Context(), input)
	if err != nil || channel.Verified || len(channel.Recipients) != 0 {
		t.Fatalf("skipped channel %+v, %v", channel, err)
	}
}
