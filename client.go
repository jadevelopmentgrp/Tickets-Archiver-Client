package archiverclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/TicketsBot/archiverclient/discord"
	"github.com/TicketsBot/common/encryption"
	"github.com/rxdn/gdl/objects/channel/message"
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

var ErrExpired = errors.New("log has expired")

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

func (c *ArchiverClient) Get(guildId uint64, ticketId int) ([]message.Message, error) {
	endpoint := fmt.Sprintf("%s/?guild=%d&id=%d", c.endpoint, guildId, ticketId)
	res, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		if res.StatusCode == 404 {
			return nil, ErrExpired
		}

		var decoded map[string]string
		if err := json.Unmarshal(body, &decoded); err != nil {
			return nil, err
		}

		return nil, errors.New(decoded["message"])
	} else {
		body, err = encryption.Decompress(body)
		if err != nil {
			return nil, err
		}

		body, err = encryption.Decrypt(c.key, body)
		if err != nil {
			return nil, err
		}

		var messages []message.Message
		if err := json.Unmarshal(body, &messages); err != nil {
			return nil, err
		}

		return messages, nil
	}
}

func (c *ArchiverClient) Store(messages []message.Message, guildId uint64, ticketId int, premium bool) error {
	reduced := discord.ReduceMessages(messages)

	data, err := json.Marshal(reduced)
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

func (c *ArchiverClient) Encode(messages []message.Message, title string) ([]byte, error) {
	encoded, err := json.Marshal(messages)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/encode?title=%s", c.endpoint, title)
	res, err := c.httpClient.Post(endpoint, "application/json", bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		var decoded map[string]string
		if err := json.NewDecoder(res.Body).Decode(&decoded); err != nil {
			return nil, err
		}

		return nil, errors.New(decoded["message"])
	} else {
		var buff bytes.Buffer
		if _, err := buff.ReadFrom(res.Body); err != nil {
			return nil, err
		}

		return buff.Bytes(), nil
	}
}
