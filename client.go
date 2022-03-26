package archiverclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/TicketsBot/common/encryption"
	"github.com/TicketsBot/logarchiver/model"
	v1 "github.com/TicketsBot/logarchiver/model/v1"
	v2 "github.com/TicketsBot/logarchiver/model/v2"
	"github.com/rxdn/gdl/objects/channel"
	"github.com/rxdn/gdl/objects/channel/message"
	"github.com/rxdn/gdl/objects/guild"
	"github.com/rxdn/gdl/objects/user"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type ArchiverClient struct {
	endpoint   string
	httpClient *http.Client
	key        []byte
}

var (
	ErrExpired  = errors.New("log has expired")
	ErrNotFound = errors.New("Transcript not found")
)

func NewArchiverClient(endpoint string, encryptionKey []byte) ArchiverClient {
	return NewArchiverClientWithTimeout(endpoint, time.Second*3, encryptionKey)
}

func NewArchiverClientWithTimeout(endpoint string, timeout time.Duration, encryptionKey []byte) ArchiverClient {
	endpoint = strings.TrimSuffix(endpoint, "/")

	return ArchiverClient{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				TLSHandshakeTimeout: time.Second * 3,
			},
		},
		key: encryptionKey,
	}
}

func (c *ArchiverClient) Get(guildId uint64, ticketId int) (v2.Transcript, error) {
	endpoint := fmt.Sprintf("%s/?guild=%d&id=%d", c.endpoint, guildId, ticketId)
	res, err := c.httpClient.Get(endpoint)
	if err != nil {
		return v2.Transcript{}, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return v2.Transcript{}, err
	}

	if res.StatusCode != 200 {
		if res.StatusCode == 404 {
			return v2.Transcript{}, ErrNotFound
		}

		var decoded map[string]string
		if err := json.Unmarshal(body, &decoded); err != nil {
			return v2.Transcript{}, err
		}

		return v2.Transcript{}, errors.New(decoded["message"])
	}

	body, err = encryption.Decompress(body)
	if err != nil {
		return v2.Transcript{}, err
	}

	body, err = encryption.Decrypt(c.key, body)
	if err != nil {
		return v2.Transcript{}, err
	}

	version := model.GetVersion(body)
	switch version {
	case model.V1:
		var messages []message.Message
		if err := json.Unmarshal(body, &messages); err != nil {
			return v2.Transcript{}, err
		}

		return v1.ConvertToV2(messages), nil
	case model.V2:
		var transcript v2.Transcript
		if err := json.Unmarshal(body, &transcript); err != nil {
			return v2.Transcript{}, err
		}

		return transcript, nil
	default:
		return v2.Transcript{}, fmt.Errorf("Unknown version %d", version)
	}
}

func (c *ArchiverClient) Store(messages []message.Message, guildId uint64, ticketId int, premium bool) error {
	transcript := v2.NewTranscript(messages, v2.NoopRetriever[user.User], v2.NoopRetriever[channel.Channel], v2.NoopRetriever[guild.Role])

	data, err := json.Marshal(transcript)
	if err != nil {
		return err
	}

	data, err = encryption.Encrypt(c.key, data)
	if err != nil {
		return err
	}

	data = encryption.Compress(data)

	endpoint := fmt.Sprintf("%s/?guild=%d&id=%d", c.endpoint, guildId, ticketId)
	if premium {
		endpoint += "&premium"
	}

	res, err := c.httpClient.Post(endpoint, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		var decoded map[string]string
		if err := json.NewDecoder(res.Body).Decode(&decoded); err != nil {
			return err
		}

		return errors.New(decoded["message"])
	}

	return nil
}

func (c *ArchiverClient) PurgeGuild(guildId uint64) error {
	endpoint := fmt.Sprintf("%s/guild/%d", c.endpoint, guildId)

	req, err := http.NewRequest(http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		var decoded map[string]string
		if err := json.NewDecoder(res.Body).Decode(&decoded); err != nil {
			return err
		}

		return errors.New(decoded["message"])
	}

	return nil
}
